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

	"github.com/srikarjy/research-orchestrator/orchestrator/internal/aletheia"
	"github.com/srikarjy/research-orchestrator/orchestrator/internal/api"
	"github.com/srikarjy/research-orchestrator/orchestrator/internal/auth"
	"github.com/srikarjy/research-orchestrator/orchestrator/internal/kernel"
	"github.com/srikarjy/research-orchestrator/orchestrator/internal/monitors"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Gateway struct {
	platform        *kernel.Platform
	router          *gin.Engine
	server          *http.Server
	workflowHandler api.WorkflowEngineService
	biolabHandler   api.BiolabMCPService
	aletheiaClient  *aletheia.Client
	authMW          *auth.Middleware
	authHandlers    *auth.Handlers
	monitorHandlers *monitors.Handlers
	stopMonitors    func()
}

func NewGateway(p *kernel.Platform) *Gateway {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	g := &Gateway{
		platform:        p,
		router:          router,
		workflowHandler: p.WorkflowEngine,
		biolabHandler:   p.BiolabMCP,
		aletheiaClient:  p.Aletheia,
	}

	if p.DB != nil {
		store, err := auth.NewStore(context.Background(), p.DB)
		if err != nil {
			// A gateway that can't set up its auth tables must not start
			// half-open: fail loudly instead of serving unprotected routes.
			p.Logger.Fatal("auth schema setup failed", zap.Error(err))
		}
		g.authMW = auth.NewMiddleware(p.Config.AuthJWTSecret, store, p.Redis, p.Logger)
		g.authHandlers = auth.NewHandlers(p.Config.AuthJWTSecret, store, p.Logger)
		if !g.authMW.Enabled() {
			p.Logger.Warn("AUTH DISABLED: AUTH_JWT_SECRET is not set — every /api/v1 route is open. Set ORCH_AUTH_JWT_SECRET before deploying anywhere reachable.")
		}
	} else {
		p.Logger.Warn("AUTH UNAVAILABLE: no database — auth routes not mounted, /api/v1 is open")
	}

	if p.DB != nil && g.aletheiaClient != nil {
		monStore, err := monitors.NewStore(context.Background(), p.DB)
		if err != nil {
			p.Logger.Fatal("monitors schema setup failed", zap.Error(err))
		}
		monService := monitors.NewService(monStore, g.aletheiaClient, p.Logger)
		g.monitorHandlers = monitors.NewHandlers(monStore, monService, p.Logger)
		// One scheduler tick per minute; each due monitor re-runs its claim
		// through the same grounded path as an ad-hoc query.
		g.stopMonitors = monService.Start(context.Background(), time.Minute)
	}

	g.setupMiddleware()
	g.setupRoutes()

	return g
}

func (g *Gateway) setupMiddleware() {
	g.router.Use(gin.Recovery())

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

	g.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	g.router.Use(func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		c.Header("X-Request-ID", reqID)
		c.Set("request_id", reqID)
		c.Next()
	})
}

func (g *Gateway) setupRoutes() {
	g.router.GET("/health", g.health)
	g.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := g.router.Group("/api/v1")
	{
		// Auth endpoints. Registration and login are the only unauthenticated
		// POSTs, and both are IP-rate-limited so they can't be used for
		// credential stuffing or account spam. Everything else under /api/v1
		// requires a JWT or API key once AUTH_JWT_SECRET is set (empty =
		// local dev, middleware passes through; see internal/auth).
		if g.authHandlers != nil {
			authGroup := v1.Group("/auth")
			{
				authGroup.POST("/register", g.authMW.RateLimit("auth", 10, time.Minute), g.authHandlers.Register)
				authGroup.POST("/login", g.authMW.RateLimit("auth", 10, time.Minute), g.authHandlers.Login)

				keys := authGroup.Group("/keys", g.authMW.Require())
				{
					keys.POST("", g.authHandlers.CreateKey)
					keys.GET("", g.authHandlers.ListKeys)
					keys.DELETE("/:id", g.authHandlers.RevokeKey)
				}
			}
			v1.Use(g.authMW.Require())
		}
		if g.platform.Config.WorkflowsEnabled {
			wf := v1.Group("/workflows")
			{
				wf.POST("", g.adaptCreateWorkflow)
				wf.GET("", g.adaptListWorkflows)
				wf.GET("/:id", g.adaptGetWorkflow)
				wf.POST("/:id/execute", g.adaptExecuteWorkflow)
				wf.GET("/:id/events", g.adaptGetEvents)
				wf.GET("/:id/steps/:stepId", g.adaptGetStep)
			}
		}

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

		if g.monitorHandlers != nil {
			mon := v1.Group("/monitors")
			{
				mon.POST("", g.monitorHandlers.Create)
				mon.GET("", g.monitorHandlers.List)
				mon.GET("/:id/history", g.monitorHandlers.History)
				mon.DELETE("/:id", g.monitorHandlers.Delete)
				// A manual check is a real Claude call — same budget-guarding
				// limit as /api/v1/query.
				if g.authMW != nil {
					mon.POST("/:id/check", g.authMW.RateLimit("query", 10, time.Minute), g.monitorHandlers.CheckNow)
				} else {
					mon.POST("/:id/check", g.monitorHandlers.CheckNow)
				}
			}
		}

		// Aletheia query endpoint - the killer query. Tightest rate limit in
		// the gateway: every request here is a real retrieval plus a real
		// Claude call (~$0.014), so this is the endpoint that can drain an
		// API key budget if left open.
		query := v1.Group("/query")
		{
			if g.authMW != nil {
				query.POST("", g.authMW.RateLimit("query", 10, time.Minute), g.adaptAletheiaQuery)
			} else {
				query.POST("", g.adaptAletheiaQuery)
			}
		}
	}

	g.router.GET("/ws/workflows/:id", g.adaptWorkflowWS)
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

func (g *Gateway) adaptWorkflowWS(c *gin.Context) {
	id := c.Param("id")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		g.platform.Logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	wf, err := g.workflowHandler.GetWorkflow(id)
	if err == nil {
		conn.WriteJSON(map[string]interface{}{
			"type": "workflow_state",
			"data": wf,
		})
	}

	events, _ := g.workflowHandler.GetEvents(id)
	for _, evt := range events {
		conn.WriteJSON(map[string]interface{}{
			"type": "event",
			"data": evt,
		})
	}

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (g *Gateway) adaptAletheiaQuery(c *gin.Context) {
	var req struct {
		Claim string `json:"claim" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if g.aletheiaClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Aletheia client not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	result, err := g.aletheiaClient.Query(ctx, req.Claim)
	if err != nil {
		g.platform.Logger.Error("Aletheia query failed", zap.Error(err), zap.String("claim", req.Claim))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
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
	if g.stopMonitors != nil {
		g.stopMonitors()
	}
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