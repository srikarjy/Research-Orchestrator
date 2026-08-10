package gateway

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/srikarjy/research-orchestrator/assayos/internal/api"
	"github.com/srikarjy/research-orchestrator/assayos/internal/kernel"
	"github.com/srikarjy/research-orchestrator/assayos/internal/ui"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Gateway struct {
	platform *kernel.Platform
	router   *gin.Engine
	server   *http.Server

	workflowHandler api.WorkflowEngineService
	biolabHandler   api.BiolabMCPService
	aletheiaHandler api.AletheiaGatewayService
	uiHandler       *ui.Handler
}

func NewGateway(p *kernel.Platform) *Gateway {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	g := &Gateway{
		platform:        p,
		router:          router,
		workflowHandler: p.WorkflowEngine,
		biolabHandler:   p.BiolabMCP,
		aletheiaHandler: p.Aletheia,
		uiHandler:       ui.NewHandler(p),
	}

	g.setupMiddleware()
	g.setupRoutes()

	return g
}

func (g *Gateway) setupMiddleware() {
	// Recovery
	g.router.Use(gin.Recovery())

	// Request logging
	g.router.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		g.platform.Logger.Info("HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	})

	// CORS
	g.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Request ID
	g.router.Use(func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		c.Header("X-Request-ID", reqID)
		c.Set("request_id", reqID)
		c.Next()
	})

	// AuthZ middleware
	g.router.Use(g.authzMiddleware())
}

func (g *Gateway) authzMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !g.platform.Config.Auth.Enabled {
			c.Next()
			return
		}

		// Skip auth for health, metrics, and static files
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/health") ||
			strings.HasPrefix(path, "/metrics") ||
			strings.HasPrefix(path, "/ui/") ||
			strings.HasPrefix(path, "/ws/") {
			c.Next()
			return
		}

		actor := c.GetHeader("X-Actor-ID")
		if actor == "" {
			actor = "anonymous"
		}

		action := strings.ToLower(c.Request.Method)
		resource := path

		allowed, reason, err := g.platform.Authz.Authorize(c.Request.Context(), actor, action, resource, map[string]interface{}{
			"method": c.Request.Method,
			"path":   path,
			"ip":     c.ClientIP(),
		})
		if err != nil {
			g.platform.Logger.Error("Authz error", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authorization failed"})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "reason": reason})
			return
		}
		c.Next()
	}
}

func (g *Gateway) setupRoutes() {
	// Health & metrics (no auth)
	g.router.GET("/health", g.health)
	g.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := g.router.Group("/api/v1")
	{
		// Workflow Engine routes (Plane 2)
		if g.platform.Config.Planes.WorkflowsEnabled {
			wf := v1.Group("/workflows")
			{
				wf.POST("", g.adaptCreateWorkflow)
				wf.GET("", g.adaptListWorkflows)
				wf.GET("/:id", g.adaptGetWorkflow)
				wf.POST("/:id/execute", g.adaptExecuteWorkflow)
				wf.GET("/:id/events", g.adaptGetEvents)
				wf.GET("/:id/steps/:stepId", g.adaptGetStep)
			}

			exec := v1.Group("/executor")
			{
				exec.GET("/calendar", g.adaptGetCalendar)
				exec.GET("/notifications", g.adaptGetNotifications)
				exec.GET("/tasks", g.adaptGetTasks)
				exec.POST("/notifications/:id/read", g.adaptMarkNotificationRead)
			}
		}

		// Biolab MCP routes (Plane 3)
		if g.platform.Config.Planes.BiolabEnabled {
			agents := v1.Group("/agents")
			{
				agents.GET("", g.adaptListAgents)
				agents.GET("/:id/status", g.adaptGetAgentStatus)
			}

			biolabWf := v1.Group("/biolab/workflows")
			{
				biolabWf.POST("", g.adaptCreateBiolabWorkflow)
				biolabWf.GET("", g.adaptListBiolabWorkflows)
				biolabWf.GET("/:id", g.adaptGetBiolabWorkflow)
				biolabWf.POST("/:id/execute", g.adaptExecuteBiolabWorkflow)
				biolabWf.DELETE("/:id", g.adaptDeleteBiolabWorkflow)
			}

			tools := v1.Group("/tools")
			{
				tools.GET("", g.adaptListTools)
				tools.GET("/:category", g.adaptListToolsByCategory)
				tools.POST("/:category/:name/execute", g.adaptExecuteTool)
				tools.GET("/:category/:name/schema", g.adaptGetToolSchema)
			}

			sandbox := v1.Group("/sandbox")
			{
				sandbox.POST("/sessions", g.adaptCreateSandboxSession)
				sandbox.GET("/sessions", g.adaptListSandboxSessions)
				sandbox.GET("/sessions/:id", g.adaptGetSandboxSession)
				sandbox.POST("/sessions/:id/execute", g.adaptExecuteSandboxExperiment)
			}

			notifs := v1.Group("/notifications")
			{
				notifs.POST("/send", g.adaptSendNotification)
			}
		}

		// Aletheia routes (Plane 1)
		if g.platform.Config.Planes.AletheiaEnabled {
			aletheia := v1.Group("/aletheia")
			{
				aletheia.POST("/investigate", g.adaptInvestigate)
				aletheia.GET("/investigate/:id/status", g.adaptGetInvestigationStatus)
				aletheia.GET("/workflows", g.adaptListAletheiaWorkflows)
			}
		}
	}

	// WebSocket endpoints
	g.router.GET("/ws/workflows/:id", g.adaptWorkflowWS)
	g.router.GET("/ws/aletheia/:id", g.adaptAletheiaWS)

	// UI static files
	if g.uiHandler != nil {
		g.router.NoRoute(g.uiHandler.ServeStatic)
	}
}

func (g *Gateway) health(c *gin.Context) {
	health := g.platform.Health(c.Request.Context())
	
	status := http.StatusOK
	for _, v := range health {
		if strings.HasPrefix(v, "unhealthy") {
			status = http.StatusServiceUnavailable
			break
		}
	}

	c.JSON(status, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "0.1.0",
		"components": health,
	})
}

// Workflow Engine adapters
func (g *Gateway) adaptCreateWorkflow(c *gin.Context) {
	var req struct {
		Name  string                 `json:"name" binding:"required"`
		Query string                 `json:"query" binding:"required"`
		Steps []*api.WorkflowStep    `json:"steps,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wf, err := g.workflowHandler.CreateWorkflow(c.Request.Context(), req.Name, req.Query, req.Steps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, wf)
}

func (g *Gateway) adaptListWorkflows(c *gin.Context) {
	workflows := g.workflowHandler.ListWorkflows()
	c.JSON(http.StatusOK, workflows)
}

func (g *Gateway) adaptGetWorkflow(c *gin.Context) {
	id := c.Param("id")
	wf, err := g.workflowHandler.GetWorkflow(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	c.JSON(http.StatusOK, wf)
}

func (g *Gateway) adaptExecuteWorkflow(c *gin.Context) {
	id := c.Param("id")
	if err := g.workflowHandler.ExecuteWorkflow(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "execution started", "workflow_id": id})
}

func (g *Gateway) adaptGetEvents(c *gin.Context) {
	id := c.Param("id")
	events, err := g.workflowHandler.GetEvents(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

func (g *Gateway) adaptGetStep(c *gin.Context) {
	id := c.Param("id")
	stepID := c.Param("stepId")
	step, err := g.workflowHandler.GetStep(id, stepID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "step not found"})
		return
	}
	c.JSON(http.StatusOK, step)
}

func (g *Gateway) adaptGetCalendar(c *gin.Context) {
	events := []map[string]interface{}{
		{"id": "cal-001", "title": "Weekly Lab Meeting", "start": time.Now().Add(24*time.Hour).Format(time.RFC3339), "end": time.Now().Add(25*time.Hour).Format(time.RFC3339), "source": "google"},
		{"id": "cal-002", "title": "Docking Run", "start": time.Now().Add(48*time.Hour).Format(time.RFC3339), "end": time.Now().Add(50*time.Hour).Format(time.RFC3339), "source": "workflow"},
	}
	c.JSON(http.StatusOK, events)
}

func (g *Gateway) adaptGetNotifications(c *gin.Context) {
	notifications := []map[string]interface{}{
		{"id": "notif-001", "type": "task_running", "title": "Protein Stability Analysis", "message": "Running...", "timestamp": time.Now().Format(time.RFC3339), "read": false, "evidence_id": "EV-0042"},
		{"id": "notif-002", "type": "review_required", "title": "Human Review Required", "message": "Contradiction detected", "timestamp": time.Now().Add(-1*time.Hour).Format(time.RFC3339), "read": false, "evidence_id": "EV-0042"},
	}
	c.JSON(http.StatusOK, notifications)
}

func (g *Gateway) adaptGetTasks(c *gin.Context) {
	tasks := []map[string]interface{}{
		{"id": "task-001", "name": "Protein Stability Prediction", "started_at": time.Now().Format(time.RFC3339), "status": "running", "progress": 0.6, "current_step": "MD relaxation", "evidence_id": "EV-0042"},
	}
	c.JSON(http.StatusOK, tasks)
}

func (g *Gateway) adaptMarkNotificationRead(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"id": id, "read": "true"})
}

// Biolab MCP adapters
func (g *Gateway) adaptListAgents(c *gin.Context) {
	agents := g.biolabHandler.ListAgents()
	c.JSON(http.StatusOK, agents)
}

func (g *Gateway) adaptGetAgentStatus(c *gin.Context) {
	id := c.Param("id")
	status, ok := g.biolabHandler.GetAgentStatus(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (g *Gateway) adaptCreateBiolabWorkflow(c *gin.Context) {
	var req struct {
		Name        string           `json:"name" binding:"required"`
		Description string           `json:"description"`
		Tasks       []api.Task       `json:"tasks" binding:"required"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wf := g.biolabHandler.CreateWorkflow(req.Name, req.Description, req.Tasks)
	wf.Metadata = req.Metadata
	c.JSON(http.StatusCreated, wf)
}

func (g *Gateway) adaptListBiolabWorkflows(c *gin.Context) {
	workflows := g.biolabHandler.ListWorkflows()
	c.JSON(http.StatusOK, workflows)
}

func (g *Gateway) adaptGetBiolabWorkflow(c *gin.Context) {
	id := c.Param("id")
	wf, ok := g.biolabHandler.GetWorkflow(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	c.JSON(http.StatusOK, wf)
}

func (g *Gateway) adaptExecuteBiolabWorkflow(c *gin.Context) {
	id := c.Param("id")
	if err := g.biolabHandler.ExecuteWorkflow(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "started", "workflow_id": id})
}

func (g *Gateway) adaptDeleteBiolabWorkflow(c *gin.Context) {
	id := c.Param("id")
	if err := g.biolabHandler.DeleteWorkflow(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (g *Gateway) adaptListTools(c *gin.Context) {
	category := c.Query("category")
	tools := g.biolabHandler.ListTools(category)
	c.JSON(http.StatusOK, tools)
}

func (g *Gateway) adaptListToolsByCategory(c *gin.Context) {
	category := c.Param("category")
	tools := g.biolabHandler.ListTools(category)
	c.JSON(http.StatusOK, tools)
}

func (g *Gateway) adaptExecuteTool(c *gin.Context) {
	category := c.Param("category")
	name := c.Param("name")

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result, err := g.biolabHandler.ExecuteTool(ctx, category, name, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (g *Gateway) adaptGetToolSchema(c *gin.Context) {
	category := c.Param("category")
	name := c.Param("name")
	schema, ok := g.biolabHandler.GetToolSchema(category, name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return
	}
	c.JSON(http.StatusOK, schema)
}

func (g *Gateway) adaptCreateSandboxSession(c *gin.Context) {
	var req struct {
		ExperimentID string                 `json:"experiment_id" binding:"required"`
		Metadata     map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := g.biolabHandler.CreateSandboxSession(c.Request.Context(), req.ExperimentID, req.Metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, session)
}

func (g *Gateway) adaptListSandboxSessions(c *gin.Context) {
	sessions := g.biolabHandler.ListSandboxSessions()
	c.JSON(http.StatusOK, sessions)
}

func (g *Gateway) adaptGetSandboxSession(c *gin.Context) {
	id := c.Param("id")
	session, ok := g.biolabHandler.GetSandboxSession(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (g *Gateway) adaptExecuteSandboxExperiment(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	session, err := g.biolabHandler.ExecuteSandboxExperiment(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (g *Gateway) adaptSendNotification(c *gin.Context) {
	var req struct {
		NotificationType string                 `json:"notification_type" binding:"required"`
		Recipients       []string               `json:"recipients" binding:"required"`
		Channels         []string               `json:"channels" binding:"required"`
		Data             map[string]interface{} `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := g.biolabHandler.SendNotification(ctx, req.NotificationType, req.Recipients, req.Channels, req.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// Aletheia adapters
func (g *Gateway) adaptInvestigate(c *gin.Context) {
	var req struct {
		Query   string                 `json:"query" binding:"required"`
		Options map[string]interface{} `json:"options"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := g.aletheiaHandler.Investigate(c.Request.Context(), req.Query, req.Options)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "aletheia service unavailable"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (g *Gateway) adaptGetInvestigationStatus(c *gin.Context) {
	id := c.Param("id")
	resp, err := g.aletheiaHandler.GetInvestigationStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "aletheia service unavailable"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (g *Gateway) adaptListAletheiaWorkflows(c *gin.Context) {
	workflows, err := g.aletheiaHandler.ListWorkflows(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "aletheia service unavailable"})
		return
	}
	c.JSON(http.StatusOK, workflows)
}

// WebSocket adapters
func (g *Gateway) adaptWorkflowWS(c *gin.Context) {
	id := c.Param("id")
	
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		g.platform.Logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	// Send current workflow state
	wf, err := g.workflowHandler.GetWorkflow(id)
	if err == nil {
		conn.WriteJSON(map[string]interface{}{
			"type": "workflow_state",
			"data": wf,
		})
	}

	// Send existing events
	events, _ := g.workflowHandler.GetEvents(id)
	for _, evt := range events {
		conn.WriteJSON(map[string]interface{}{
			"type": "event",
			"data": evt,
		})
	}

	// Keep alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (g *Gateway) adaptAletheiaWS(c *gin.Context) {
	id := c.Param("id")
	
	// Connect to Aletheia WebSocket
	targetURL := "ws://" + g.platform.Config.Planes.AletheiaEndpoint[7:] + "/ws/" + id
	
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		g.platform.Logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	// Connect to upstream
	upstream, _, err := websocket.DefaultDialer.Dial(targetURL, nil)
	if err != nil {
		g.platform.Logger.Error("Upstream WebSocket connect failed", zap.Error(err), zap.String("url", targetURL))
		conn.WriteJSON(map[string]interface{}{"type": "error", "message": "upstream unavailable"})
		return
	}
	defer upstream.Close()

	// Bidirectional proxy
	errChan := make(chan error, 2)
	
	go func() {
		for {
			_, msg, err := upstream.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				errChan <- err
				return
			}
		}
	}()
	
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if err := upstream.WriteMessage(websocket.TextMessage, msg); err != nil {
				errChan <- err
				return
			}
		}
	}()
	
	<-errChan
}

func (g *Gateway) Start() error {
	g.server = &http.Server{
		Addr:         ":" + strconv.Itoa(g.platform.Config.Server.Port),
		Handler:      g.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	g.platform.Logger.Info("Starting API Gateway", zap.String("addr", g.server.Addr))
	return g.server.ListenAndServe()
}

func (g *Gateway) Stop(ctx context.Context) error {
	g.platform.Logger.Info("Stopping API Gateway...")
	return g.server.Shutdown(ctx)
}

func generateRequestID() string {
	return "req_" + time.Now().UTC().Format("20060102T150405.000000") + "_" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}