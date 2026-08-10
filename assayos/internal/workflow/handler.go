package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/srikarjy/research-orchestrator/assayos/internal/kernel"
	"github.com/srikarjy/research-orchestrator/workflow-engine/internal/engine"
	"github.com/srikarjy/research-orchestrator/workflow-engine/pkg/eventlog"
)

type Handler struct {
	platform *kernel.Platform
	engine   *engine.Engine
	store    eventlog.Store
	logger   *zap.Logger
	upgrader websocket.Upgrader
}

func NewHandler(p *kernel.Platform) *Handler {
	store := eventlog.NewMemoryStore()
	eng := engine.NewEngine(store)
	
	// Register mock executors
	eng.RegisterExecutor(&MockExecutor{ToolName: "PubMed", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "UniProt", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "ChEMBL", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "ProteinStabilityPredictor", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "Docking", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "EvidenceMerge", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "Critic", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "StructureViewer", Category: "visualizer"})

	// Set event callback for WebSocket broadcasting
	eng.SetEventCallback(func(workflowID string, event *eventlog.Event) {
		// Broadcast to WebSocket clients
		// Implementation in WebSocket handler
	})

	h := &Handler{
		platform: p,
		engine:   eng,
		store:    store,
		logger:   p.Logger.Named("workflow-handler"),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	return h
}

func (h *Handler) CreateWorkflow(c *gin.Context) {
	var req struct {
		Name  string                 `json:"name" binding:"required"`
		Query string                 `json:"query" binding:"required"`
		Steps []*engine.Step         `json:"steps,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	steps := req.Steps
	if len(steps) == 0 {
		_, steps = engine.BuildDemoWorkflow(req.Query)
	}

	wf, err := h.engine.CreateWorkflow(c.Request.Context(), req.Name, req.Query, steps)
	if err != nil {
		h.logger.Error("Create workflow failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create workflow"})
		return
	}

	c.JSON(http.StatusCreated, wf)
}

func (h *Handler) ListWorkflows(c *gin.Context) {
	workflows := h.engine.GetAllWorkflows()
	result := make([]*engine.Workflow, 0, len(workflows))
	for _, wf := range workflows {
		result = append(result, wf)
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetWorkflow(c *gin.Context) {
	id := c.Param("id")
	wf, err := h.engine.GetWorkflow(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	c.JSON(http.StatusOK, wf)
}

func (h *Handler) ExecuteWorkflow(c *gin.Context) {
	id := c.Param("id")
	go func() {
		ctx := context.Background()
		if err := h.engine.ExecuteWorkflow(ctx, id); err != nil {
			h.logger.Error("Workflow execution failed", zap.String("workflow_id", id), zap.Error(err))
		}
	}()
	c.JSON(http.StatusAccepted, gin.H{"status": "execution started", "workflow_id": id})
}

func (h *Handler) GetEvents(c *gin.Context) {
	id := c.Param("id")
	events, err := h.engine.GetWorkflowEvents(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch events"})
		return
	}
	c.JSON(http.StatusOK, events)
}

func (h *Handler) GetStep(c *gin.Context) {
	id := c.Param("id")
	stepID := c.Param("stepId")
	
	wf, err := h.engine.GetWorkflow(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	for _, step := range wf.Steps {
		if step.ID == stepID {
			c.JSON(http.StatusOK, step)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "step not found"})
}

func (h *Handler) GetCalendar(c *gin.Context) {
	events := []map[string]interface{}{
		{"id": "cal-001", "title": "Weekly Lab Meeting", "start": time.Now().Add(24*time.Hour).Format(time.RFC3339), "end": time.Now().Add(25*time.Hour).Format(time.RFC3339), "source": "google"},
		{"id": "cal-002", "title": "Docking Run", "start": time.Now().Add(48*time.Hour).Format(time.RFC3339), "end": time.Now().Add(50*time.Hour).Format(time.RFC3339), "source": "workflow"},
	}
	c.JSON(http.StatusOK, events)
}

func (h *Handler) GetNotifications(c *gin.Context) {
	notifications := []map[string]interface{}{
		{"id": "notif-001", "type": "task_running", "title": "Protein Stability Analysis", "message": "Running...", "timestamp": time.Now().Format(time.RFC3339), "read": false, "evidence_id": "EV-0042"},
		{"id": "notif-002", "type": "review_required", "title": "Human Review Required", "message": "Contradiction detected", "timestamp": time.Now().Add(-1*time.Hour).Format(time.RFC3339), "read": false, "evidence_id": "EV-0042"},
	}
	c.JSON(http.StatusOK, notifications)
}

func (h *Handler) GetTasks(c *gin.Context) {
	tasks := []map[string]interface{}{
		{"id": "task-001", "name": "Protein Stability Prediction", "started_at": time.Now().Format(time.RFC3339), "status": "running", "progress": 0.6, "current_step": "MD relaxation", "evidence_id": "EV-0042"},
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *Handler) MarkNotificationRead(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"id": id, "read": "true"})
}

func (h *Handler) WebSocketHandler(c *gin.Context) {
	id := c.Param("id")
	
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	// Send current workflow state
	wf, err := h.engine.GetWorkflow(id)
	if err == nil {
		conn.WriteJSON(map[string]interface{}{
			"type": "workflow_state",
			"data": wf,
		})
	}

	// Send existing events
	events, _ := h.engine.GetWorkflowEvents(id)
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

type MockExecutor struct {
	ToolName string
	Category string
}

func (m *MockExecutor) Execute(ctx context.Context, step *engine.Step) (map[string]interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}
	return map[string]interface{}{
		"tool":      m.ToolName,
		"status":    "completed",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"simulated": true,
	}, nil
}

func (m *MockExecutor) GetToolName() string { return m.ToolName }
func (m *MockExecutor) GetCategory() string { return m.Category }