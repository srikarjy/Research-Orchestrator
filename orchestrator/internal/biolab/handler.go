package biolab

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/srikarjy/research-orchestrator/orchestrator/internal/kernel"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/agents"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/mcp"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/sandbox"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/shared"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/tools/analyzers"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/tools/retrievers"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/tools/visualizers"
)

type Handler struct {
	platform   *kernel.Platform
	registry   *mcp.ToolRegistry
	msgBus     *agents.InMemoryMessageBus
	sandbox    *sandbox.Sandbox
	orchestrator *agents.Orchestrator
	logger     *zap.Logger
}

func NewHandler(p *kernel.Platform) *Handler {
	logger := p.Logger.Named("biolab-handler")

	registry := mcp.NewToolRegistry()
	registry.Register(retrievers.NewPubMedRetriever())
	registry.Register(retrievers.NewUniProtRetriever())
	registry.Register(retrievers.NewChEMBLRetriever())
	registry.Register(retrievers.NewPDBRetriever())
	registry.Register(retrievers.NewKEGGRetriever())
	registry.Register(retrievers.NewBindingDBRetriever())
	registry.Register(analyzers.NewProteinStabilityPredictor())
	registry.Register(analyzers.NewDockingAnalyzer())
	registry.Register(analyzers.NewEvidenceMerge())
	registry.Register(analyzers.NewCriticAgent())
	registry.Register(visualizers.NewStructureViewer())
	registry.Register(visualizers.NewMoleculeViewer())

	msgBus := agents.NewInMemoryMessageBus()

	sandboxConfig := sandbox.SandboxConfig{
		BasePath:       "/tmp/assayos-sandbox",
		MaxConcurrent:  10,
		DefaultTimeout: 30 * time.Minute,
	}
	sb := sandbox.NewSandbox(sandboxConfig, logger)

	factory := agents.NewAgentFactory(msgBus, registry, sb, logger, agents.EmailConfig{})
	agentList, _ := factory.CreateAllAgents()
	orchestrator := factory.CreateOrchestrator()
	for _, agent := range agentList {
		orchestrator.RegisterAgent(agent)
	}

	return &Handler{
		platform:     p,
		registry:     registry,
		msgBus:       msgBus,
		sandbox:      sb,
		orchestrator: orchestrator,
		logger:       logger,
	}
}

func (h *Handler) ListAgents(c *gin.Context) {
	agentList := h.orchestrator.ListAgents()
	result := make([]map[string]interface{}, len(agentList))
	for i, a := range agentList {
		result[i] = map[string]interface{}{
			"id":           a.ID(),
			"name":         a.Name(),
			"description":  a.Description(),
			"capabilities": a.Capabilities(),
			"status":       a.Status(),
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetAgentStatus(c *gin.Context) {
	id := agents.AgentID(c.Param("id"))
	agent, ok := h.orchestrator.GetAgent(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{
		"id":     agent.ID(),
		"name":   agent.Name(),
		"status": agent.Status(),
	})
}

func (h *Handler) CreateWorkflow(c *gin.Context) {
	var req struct {
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Tasks       []agents.Task          `json:"tasks" binding:"required"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wf := h.orchestrator.CreateWorkflow(req.Name, req.Description, req.Tasks)
	wf.Metadata = req.Metadata
	c.JSON(http.StatusCreated, wf)
}

func (h *Handler) ListWorkflows(c *gin.Context) {
	workflows := h.orchestrator.ListWorkflows()
	c.JSON(http.StatusOK, workflows)
}

func (h *Handler) GetWorkflow(c *gin.Context) {
	id := c.Param("id")
	wf, ok := h.orchestrator.GetWorkflow(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	c.JSON(http.StatusOK, wf)
}

func (h *Handler) ExecuteWorkflow(c *gin.Context) {
	id := c.Param("id")
	go func() {
		if err := h.orchestrator.ExecuteWorkflow(context.Background(), id); err != nil {
			h.logger.Error("Workflow execution failed", zap.String("id", id), zap.Error(err))
		}
	}()
	c.JSON(http.StatusAccepted, gin.H{"status": "started", "workflow_id": id})
}

func (h *Handler) DeleteWorkflow(c *gin.Context) {
	id := c.Param("id")
	if err := h.orchestrator.CancelWorkflow(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListTools(c *gin.Context) {
	category := c.Query("category")
	var tools []mcp.Tool
	if category != "" {
		tools = h.registry.ListByCategory(category)
	} else {
		tools = h.registry.List()
	}
	result := make([]map[string]interface{}, len(tools))
	for i, t := range tools {
		result[i] = map[string]interface{}{
			"name":         t.Name(),
			"category":     t.Category(),
			"description":  t.Description(),
			"input_schema": t.InputSchema(),
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ListToolsByCategory(c *gin.Context) {
	category := c.Param("category")
	tools := h.registry.ListByCategory(category)
	result := make([]map[string]interface{}, len(tools))
	for i, t := range tools {
		result[i] = map[string]interface{}{
			"name":         t.Name(),
			"category":     t.Category(),
			"description":  t.Description(),
			"input_schema": t.InputSchema(),
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ExecuteTool(c *gin.Context) {
	category := c.Param("category")
	name := c.Param("name")

	tool, ok := h.registry.Get(category, name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetToolSchema(c *gin.Context) {
	category := c.Param("category")
	name := c.Param("name")
	tool, ok := h.registry.Get(category, name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return
	}
	c.JSON(http.StatusOK, tool.InputSchema())
}

func (h *Handler) CreateSandboxSession(c *gin.Context) {
	var req struct {
		ExperimentID string                 `json:"experiment_id" binding:"required"`
		Metadata     map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.sandbox.CreateSession(c.Request.Context(), req.ExperimentID, req.Metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, session)
}

func (h *Handler) ListSandboxSessions(c *gin.Context) {
	sessions := h.sandbox.ListSessions()
	c.JSON(http.StatusOK, sessions)
}

func (h *Handler) GetSandboxSession(c *gin.Context) {
	id := c.Param("id")
	session, ok := h.sandbox.GetSession(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *Handler) ExecuteSandboxExperiment(c *gin.Context) {
	id := c.Param("id")
	var req shared.ExperimentSpec
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	session, err := h.sandbox.RunExperiment(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *Handler) SendNotification(c *gin.Context) {
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

	notifierAgent, ok := h.orchestrator.GetAgent(agents.AgentNotifier)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "notifier agent not found"})
		return
	}

	task := agents.Task{
		ID:   uuid.New().String(),
		Type: "notify",
		Input: map[string]interface{}{
			"notification_type": req.NotificationType,
			"recipients":        req.Recipients,
			"channels":          req.Channels,
			"data":              req.Data,
		},
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := notifierAgent.Execute(ctx, task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result.Output)
}