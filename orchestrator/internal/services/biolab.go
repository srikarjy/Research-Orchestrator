package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/srikarjy/research-orchestrator/orchestrator/internal/api"
	"go.uber.org/zap"
)

// Tool interface - matches biolab-mcp-server internal/mcp
type Tool interface {
	Name() string
	Category() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

func (r *ToolRegistry) Register(tool Tool) {
	key := tool.Category() + ":" + tool.Name()
	r.tools[key] = tool
}

func (r *ToolRegistry) Get(category, name string) (Tool, bool) {
	tool, ok := r.tools[category+":"+name]
	return tool, ok
}

func (r *ToolRegistry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *ToolRegistry) ListByCategory(category string) []Tool {
	var tools []Tool
	for _, t := range r.tools {
		if t.Category() == category {
			tools = append(tools, t)
		}
	}
	return tools
}

// BiolabConfig holds configuration for the Biolab service
type BiolabConfig struct {
	SandboxPath     string
	MaxConcurrent   int
	DefaultTimeout  time.Duration
	BiolabMCPURL    string
}

// Agent interface
type Agent interface {
	ID() string
	Name() string
	Description() string
	Capabilities() []string
	Status() string
	Execute(ctx context.Context, task api.Task) (api.TaskResult, error)
	HandleMessage(ctx context.Context, msg Message) (Message, error)
}

type AgentID string

type AgentStatus string

const (
	AgentStatusIdle     AgentStatus = "idle"
	AgentStatusRunning  AgentStatus = "running"
	AgentStatusBusy     AgentStatus = "busy"
	AgentStatusError    AgentStatus = "error"
)

// Task represents a work unit for an agent
type Task struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Description  string                 `json:"description"`
	Input        map[string]interface{} `json:"input"`
	Priority     int                    `json:"priority"`
	Dependencies []string               `json:"dependencies"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID    string                 `json:"task_id"`
	AgentID   string                 `json:"agent_id"`
	Status    string                 `json:"status"`
	Output    map[string]interface{} `json:"output"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Artifacts []Artifact             `json:"artifacts"`
}

type Artifact struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Content  interface{}            `json:"content"`
	Path     string                 `json:"path,omitempty"`
	Metadata map[string]interface{} `json:"metadata"`
}

type Message struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Payload   []byte                 `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
	TraceID   string                 `json:"trace_id"`
}

const (
	MessageTypeTask   = "task"
	MessageTypeResult = "result"
	MessageTypeEvent  = "event"
)

type MessageBus interface {
	Publish(ctx context.Context, topic string, msg Message) error
	Subscribe(topic string, handler func(context.Context, Message) (Message, error))
	Unsubscribe(topic string)
}

// In-memory message bus implementation
type InMemoryMessageBus struct {
	mu       sync.RWMutex
	handlers map[string]func(context.Context, Message) (Message, error)
}

func NewInMemoryMessageBus() *InMemoryMessageBus {
	return &InMemoryMessageBus{handlers: make(map[string]func(context.Context, Message) (Message, error))}
}

func (m *InMemoryMessageBus) Publish(ctx context.Context, topic string, msg Message) error {
	m.mu.RLock()
	handler, ok := m.handlers[topic]
	m.mu.RUnlock()
	if ok {
		_, err := handler(ctx, msg)
		return err
	}
	return nil
}

func (m *InMemoryMessageBus) Subscribe(topic string, handler func(context.Context, Message) (Message, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[topic] = handler
}

func (m *InMemoryMessageBus) Unsubscribe(topic string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.handlers, topic)
}

// AgentConfig holds agent configuration
type AgentConfig struct {
	ID          AgentID
	Name        string
	Description string
	Capabilities []string
	MaxRetries  int
	Timeout     time.Duration
}

type BaseAgent struct {
	config   AgentConfig
	msgBus   MessageBus
	status   AgentStatus
	mu       sync.RWMutex
}

func NewBaseAgent(config AgentConfig, msgBus MessageBus) *BaseAgent {
	return &BaseAgent{
		config: config,
		msgBus: msgBus,
		status: AgentStatusIdle,
	}
}

func (b *BaseAgent) ID() AgentID           { return b.config.ID }
func (b *BaseAgent) Name() string          { return b.config.Name }
func (b *BaseAgent) Description() string   { return b.config.Description }
func (b *BaseAgent) Capabilities() []string { return b.config.Capabilities }
func (b *BaseAgent) Status() AgentStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}
func (b *BaseAgent) SetStatus(s AgentStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = s
}

func (b *BaseAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	return Message{}, nil
}

func (b *BaseAgent) Execute(ctx context.Context, task api.Task) (api.TaskResult, error) {
	return api.TaskResult{}, nil
}

// MockTool for testing
type MockTool struct {
	name     string
	category string
	desc     string
}

func (m *MockTool) Name() string        { return m.name }
func (m *MockTool) Category() string    { return m.category }
func (m *MockTool) Description() string { return m.desc }
func (m *MockTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]interface{}{"type": "string"}}}
}
func (m *MockTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool":      m.name,
		"status":    "completed",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"simulated": true,
	}, nil
}

// Sandbox for experiment execution
type SandboxConfig struct {
	BasePath       string
	MaxConcurrent  int
	DefaultTimeout time.Duration
}

type SandboxSession struct {
	ID           string                 `json:"id"`
	ExperimentID string                 `json:"experiment_id"`
	Status       string                 `json:"status"`
	WorkDir      string                 `json:"work_dir"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
}

type Sandbox struct {
	config  SandboxConfig
	logger  *zap.Logger
	mu      sync.RWMutex
	sessions map[string]*SandboxSession
}

func NewSandbox(config SandboxConfig, logger *zap.Logger) *Sandbox {
	return &Sandbox{
		config:   config,
		logger:   logger,
		sessions: make(map[string]*SandboxSession),
	}
}

func (s *Sandbox) CreateSession(ctx context.Context, experimentID string, metadata map[string]interface{}) (*SandboxSession, error) {
	session := &SandboxSession{
		ID:           uuid.New().String(),
		ExperimentID: experimentID,
		Status:       "created",
		WorkDir:      s.config.BasePath + "/" + experimentID,
		Metadata:     metadata,
		CreatedAt:    time.Now(),
	}
	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()
	return session, nil
}

func (s *Sandbox) ListSessions() []*SandboxSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*SandboxSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		result = append(result, sess)
	}
	return result
}

func (s *Sandbox) GetSession(id string) (*SandboxSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *Sandbox) CleanupSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

func (s *Sandbox) RunExperiment(ctx context.Context, sessionID string, spec ExperimentSpec) (*SandboxSession, error) {
	session, ok := s.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	session.Status = "running"
	// Simulate work
	time.Sleep(100 * time.Millisecond)
	session.Status = "completed"
	return session, nil
}

// ExperimentSpec for sandbox execution
type ExperimentSpec struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
	Resources  map[string]interface{} `json:"resources"`
}

// Agent Factory
type EmailConfig struct {
	SMTPHost    string
	SMTPPort    int
	Username    string
	Password    string
	FromAddress string
	FromName    string
	UseTLS      bool
}

type AgentFactory struct {
	msgBus      MessageBus
	toolRegistry *ToolRegistry
	sandbox     *Sandbox
	logger      *zap.Logger
	emailConfig EmailConfig
}

func NewAgentFactory(msgBus MessageBus, toolRegistry *ToolRegistry, sandbox *Sandbox, logger *zap.Logger, emailConfig EmailConfig) *AgentFactory {
	return &AgentFactory{
		msgBus:       msgBus,
		toolRegistry: toolRegistry,
		sandbox:      sandbox,
		logger:       logger,
		emailConfig:  emailConfig,
	}
}

func (f *AgentFactory) CreateAllAgents() ([]Agent, error) {
	agents := make([]Agent, 0)

	// Planner Agent
	planner := &MockAgent{id: "planner", name: "Experiment Planner", caps: []string{"plan", "design", "hypothesis_generation", "resource_allocation"}}
	agents = append(agents, planner)

	// Researcher Agent
	researcher := &MockAgent{id: "researcher", name: "Literature Researcher", caps: []string{"research", "literature_search", "evidence_retrieval", "synthesis"}}
	agents = append(agents, researcher)

	// Executor Agent
	executor := &MockAgent{id: "executor", name: "Experiment Executor", caps: []string{"compute", "experiment", "docking", "md_simulation", "stability_prediction", "wetlab_protocol", "analyze", "synthesize"}}
	agents = append(agents, executor)

	// Validator Agent
	validator := &MockAgent{id: "validator", name: "Result Validator", caps: []string{"validate", "statistical_validation", "reproducibility", "orthogonal_validation", "quality_check"}}
	agents = append(agents, validator)

	// Critic Agent
	critic := &MockAgent{id: "critic", name: "Evidence Critic", caps: []string{"critique", "evidence_analysis", "contradiction_detection"}}
	agents = append(agents, critic)

	// Notifier Agent
	notifier := &MockAgent{id: "notifier", name: "Notification Agent", caps: []string{"notify", "email", "webhook", "slack", "alerting"}}
	agents = append(agents, notifier)

	// Clinical Trial Agent
	clinicalTrial := &MockAgent{id: "clinical_trial", name: "Clinical Trial Designer", caps: []string{"clinical_trial_design", "protocol_generation", "sample_size_calculation", "regulatory_pathway", "adaptive_design"}}
	agents = append(agents, clinicalTrial)

	// Regulatory Agent
	regulatory := &MockAgent{id: "regulatory", name: "Regulatory Compliance", caps: []string{"21cfr11", "gxp", "ich", "gdpr", "compliance_audit", "submission_readiness"}}
	agents = append(agents, regulatory)

	// Biomarker Agent
	biomarker := &MockAgent{id: "biomarker", name: "Biomarker Discovery", caps: []string{"biomarker_discovery", "biomarker_validation", "biomarker_qualification", "companion_dx", "monitoring_panel"}}
	agents = append(agents, biomarker)

	return agents, nil
}

// MockAgent implements Agent interface for testing
type MockAgent struct {
	id   string
	name string
	caps []string
}

func (m *MockAgent) ID() string          { return m.id }
func (m *MockAgent) Name() string        { return m.name }
func (m *MockAgent) Description() string { return m.name + " agent" }
func (m *MockAgent) Capabilities() []string { return m.caps }
func (m *MockAgent) Status() string      { return "idle" }
func (m *MockAgent) Execute(ctx context.Context, task api.Task) (api.TaskResult, error) {
	return api.TaskResult{
		TaskID:  task.ID,
		AgentID: m.id,
		Status:  "completed",
		Output:  map[string]interface{}{"simulated": true},
	}, nil
}
func (m *MockAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	return Message{}, nil
}

// Orchestrator for managing workflows
type Orchestrator struct {
	mu        sync.RWMutex
	agents    map[AgentID]Agent
	workflows map[string]*api.BiolabWorkflow
	msgBus    MessageBus
	logger    *zap.Logger
}

func NewOrchestrator(msgBus MessageBus, logger *zap.Logger) *Orchestrator {
	return &Orchestrator{
		agents:    make(map[AgentID]Agent),
		workflows: make(map[string]*api.BiolabWorkflow),
		msgBus:    msgBus,
		logger:    logger,
	}
}

func (o *Orchestrator) RegisterAgent(agent Agent) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, exists := o.agents[AgentID(agent.ID())]; exists {
		return fmt.Errorf("agent %s already registered", agent.ID())
	}
	o.agents[AgentID(agent.ID())] = agent
	o.msgBus.Subscribe(agent.ID(), agent.HandleMessage)
	o.logger.Info("Agent registered", zap.String("agent", agent.ID()))
	return nil
}

func (o *Orchestrator) UnregisterAgent(id AgentID) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, ok := o.agents[id]
	if !ok {
		return fmt.Errorf("agent not found")
	}
	o.msgBus.Unsubscribe(string(id))
	delete(o.agents, id)
	o.logger.Info("Agent unregistered", zap.String("agent", string(id)))
	return nil
}

func (o *Orchestrator) GetAgent(id AgentID) (Agent, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	agent, ok := o.agents[id]
	return agent, ok
}

func (o *Orchestrator) ListAgents() []Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()
	agents := make([]Agent, 0, len(o.agents))
	for _, a := range o.agents {
		agents = append(agents, a)
	}
	return agents
}

func (o *Orchestrator) CreateWorkflow(name, description string, tasks []api.Task) *api.BiolabWorkflow {
	workflow := &api.BiolabWorkflow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Tasks:       tasks,
		Status:      "pending",
		Results:     make(map[string]api.TaskResult),
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}
	return workflow
}

func (o *Orchestrator) GetWorkflow(id string) (*api.BiolabWorkflow, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	w, ok := o.workflows[id]
	return w, ok
}

func (o *Orchestrator) ExecuteWorkflow(ctx context.Context, workflowID string) error {
	o.mu.Lock()
	workflow, ok := o.workflows[workflowID]
	if !ok {
		o.mu.Unlock()
		return fmt.Errorf("workflow not found: %s", workflowID)
	}
	if workflow.Status == "running" {
		o.mu.Unlock()
		return fmt.Errorf("workflow already running")
	}
	workflow.Status = "running"
	now := time.Now()
	workflow.StartedAt = &now
	o.mu.Unlock()

	o.logger.Info("Starting workflow", zap.String("id", workflowID), zap.String("name", workflow.Name))

	completed := make(map[string]bool)
	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			o.failWorkflow(workflowID, ctx.Err())
			return ctx.Err()
		default:
		}

		allDone := true
		for _, task := range workflow.Tasks {
			mu.Lock()
			done := completed[task.ID]
			mu.Unlock()

			if done {
				continue
			}

			depsMet := true
			for _, dep := range task.Dependencies {
				mu.Lock()
				depDone := completed[dep]
				mu.Unlock()
				if !depDone {
					depsMet = false
					break
				}
			}

			if !depsMet {
				allDone = false
				continue
			}

			agent := o.findAgentForTask(task)
			if agent == nil {
				o.failWorkflow(workflowID, fmt.Errorf("no agent for task %s", task.Type))
				return fmt.Errorf("no agent for task type: %s", task.Type)
			}

			allDone = false
			go func(t api.Task, a Agent) {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()

				result, err := a.Execute(ctx, t)

				mu.Lock()
				completed[t.ID] = true
				o.mu.Lock()
				if workflow, ok := o.workflows[workflowID]; ok {
					workflow.Results[t.ID] = result
				}
				o.mu.Unlock()
				mu.Unlock()

				if err != nil {
					o.logger.Error("Task failed", zap.String("task", t.ID), zap.Error(err))
				} else {
					o.logger.Info("Task completed", zap.String("task", t.ID), zap.String("agent", a.ID()))
				}
			}(task, agent)
		}

		mu.Lock()
		doneCount := 0
		for _, done := range completed {
			if done {
				doneCount++
			}
		}
		mu.Unlock()

		if doneCount == len(workflow.Tasks) {
			break
		}

		if allDone {
			time.Sleep(100 * time.Millisecond)
		}
	}

	o.completeWorkflow(workflowID)
	return nil
}

func (o *Orchestrator) findAgentForTask(task api.Task) Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, agent := range o.agents {
		for _, cap := range agent.Capabilities() {
			if cap == task.Type {
				if agent.Status() == "idle" {
					return agent
				}
			}
		}
	}
	return nil
}

func (o *Orchestrator) completeWorkflow(workflowID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	workflow, ok := o.workflows[workflowID]
	if !ok {
		return
	}

	workflow.Status = "completed"
	now := time.Now()
	workflow.CompletedAt = &now

	o.logger.Info("Workflow completed", zap.String("id", workflowID))
}

func (o *Orchestrator) failWorkflow(workflowID string, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	workflow, ok := o.workflows[workflowID]
	if !ok {
		return
	}

	workflow.Status = "failed"
	now := time.Now()
	workflow.CompletedAt = &now
	if workflow.Metadata == nil {
		workflow.Metadata = make(map[string]interface{})
	}
	workflow.Metadata["error"] = err.Error()

	o.logger.Error("Workflow failed", zap.String("id", workflowID), zap.Error(err))
}

func (o *Orchestrator) CancelWorkflow(workflowID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	workflow, ok := o.workflows[workflowID]
	if !ok {
		return fmt.Errorf("workflow not found")
	}

	if workflow.Status != "running" && workflow.Status != "pending" {
		return fmt.Errorf("workflow not running")
	}

	workflow.Status = "cancelled"
	now := time.Now()
	workflow.CompletedAt = &now

	return nil
}

func (o *Orchestrator) ListWorkflows() []*api.BiolabWorkflow {
	o.mu.RLock()
	defer o.mu.RUnlock()

	workflows := make([]*api.BiolabWorkflow, 0, len(o.workflows))
	for _, w := range o.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

func (o *Orchestrator) GetAgentStatus() map[AgentID]string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	status := make(map[AgentID]string)
	for id, agent := range o.agents {
		status[id] = agent.Status()
	}
	return status
}

// BiolabMCPService wraps the Orchestrator to implement api.BiolabMCPService and kernel.Service
type BiolabMCPService struct {
	logger        *zap.Logger
	orchestrator  *Orchestrator
	toolRegistry  *ToolRegistry
	sandbox       *Sandbox
	biolabMCPURL  string
	httpClient    *http.Client
}

func NewBiolabMCPService(logger *zap.Logger, config *BiolabConfig) *BiolabMCPService {
	if config == nil {
		config = &BiolabConfig{
			SandboxPath:    "/tmp/assayos-sandbox",
			MaxConcurrent:  10,
			DefaultTimeout: 30 * time.Minute,
			BiolabMCPURL:   getEnv("ASSAYOS_BIOLAB_MCP_URL", "http://biolab-mcp:8081"),
		}
	}
	logger = logger.Named("biolab-mcp")

	registry := NewToolRegistry()
	
	// Always register mock tools for local tool listing/schema
	registry.Register(&MockTool{name: "PubMed", category: "retriever", desc: "Search PubMed for biomedical literature"})
	registry.Register(&MockTool{name: "UniProt", category: "retriever", desc: "Query UniProt for protein sequences and annotations"})
	registry.Register(&MockTool{name: "ChEMBL", category: "retriever", desc: "Query ChEMBL for bioactive molecules"})
	registry.Register(&MockTool{name: "PDB", category: "retriever", desc: "Query RCSB PDB for protein structures"})
	registry.Register(&MockTool{name: "KEGG", category: "retriever", desc: "Query KEGG for pathways, genes, and compounds"})
	registry.Register(&MockTool{name: "BindingDB", category: "retriever", desc: "Query BindingDB for protein-ligand binding affinities"})
	
	// Register analyzers (mock for now)
	registry.Register(&MockTool{name: "ProteinStabilityPredictor", category: "analyzer", desc: "Predict protein stability changes from mutations"})
	registry.Register(&MockTool{name: "Docking", category: "analyzer", desc: "Molecular docking with AutoDock Vina"})
	registry.Register(&MockTool{name: "EvidenceMerge", category: "analyzer", desc: "Merge and deduplicate evidence from multiple sources"})
	registry.Register(&MockTool{name: "Critic", category: "analyzer", desc: "Critic agent: finds contradictions, assesses evidence quality"})
	
	// Register visualizers (mock for now)
	registry.Register(&MockTool{name: "StructureViewer", category: "visualizer", desc: "Generate 3Dmol.js-compatible structure view data"})
	registry.Register(&MockTool{name: "MoleculeViewer", category: "visualizer", desc: "Generate RDKit.js-compatible 2D structure from SMILES"})

	msgBus := NewInMemoryMessageBus()
	sandboxConfig := SandboxConfig{
		BasePath:       config.SandboxPath,
		MaxConcurrent:  config.MaxConcurrent,
		DefaultTimeout: config.DefaultTimeout,
	}
	sb := NewSandbox(sandboxConfig, logger)

	factory := NewAgentFactory(msgBus, registry, sb, logger, EmailConfig{})

	agentList, err := factory.CreateAllAgents()
	if err != nil {
		logger.Error("Failed to create agents", zap.Error(err))
	}

	orchestrator := NewOrchestrator(msgBus, logger)
	for _, agent := range agentList {
		if err := orchestrator.RegisterAgent(agent); err != nil {
			logger.Error("Failed to register agent", zap.Error(err), zap.String("agent", agent.Name()))
		}
	}

	return &BiolabMCPService{
		logger:        logger,
		orchestrator:  orchestrator,
		toolRegistry:  registry,
		sandbox:       sb,
		biolabMCPURL:  config.BiolabMCPURL,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *BiolabMCPService) Name() string { return "biolab-mcp" }

func (s *BiolabMCPService) Start(ctx context.Context) error {
	s.logger.Info("Biolab MCP service started", zap.Int("tools", len(s.toolRegistry.List())))
	return nil
}

func (s *BiolabMCPService) Stop(ctx context.Context) error {
	s.logger.Info("Biolab MCP service stopped")
	return nil
}

func (s *BiolabMCPService) Health(ctx context.Context) error {
	return nil
}

// Implement api.BiolabMCPService interface
func (s *BiolabMCPService) ListAgents() []api.AgentInfo {
	agentList := s.orchestrator.ListAgents()
	result := make([]api.AgentInfo, len(agentList))
	for i, a := range agentList {
		result[i] = api.AgentInfo{
			ID:           a.ID(),
			Name:         a.Name(),
			Description:  a.Description(),
			Capabilities: a.Capabilities(),
			Status:       a.Status(),
		}
	}
	return result
}

func (s *BiolabMCPService) GetAgentStatus(id string) (api.AgentStatus, bool) {
	agent, ok := s.orchestrator.GetAgent(AgentID(id))
	if !ok {
		return api.AgentStatus{}, false
	}
	return api.AgentStatus{ID: agent.ID(), Name: agent.Name(), Status: agent.Status()}, true
}

func (s *BiolabMCPService) CreateWorkflow(name, description string, tasks []api.Task) *api.BiolabWorkflow {
	return s.orchestrator.CreateWorkflow(name, description, tasks)
}

func (s *BiolabMCPService) GetWorkflow(id string) (*api.BiolabWorkflow, bool) {
	return s.orchestrator.GetWorkflow(id)
}

func (s *BiolabMCPService) ListWorkflows() []*api.BiolabWorkflow {
	return s.orchestrator.ListWorkflows()
}

func (s *BiolabMCPService) ExecuteWorkflow(ctx context.Context, id string) error {
	return s.orchestrator.ExecuteWorkflow(ctx, id)
}

func (s *BiolabMCPService) DeleteWorkflow(id string) error {
	s.orchestrator.mu.Lock()
	defer s.orchestrator.mu.Unlock()
	delete(s.orchestrator.workflows, id)
	return nil
}

func (s *BiolabMCPService) ListTools(category string) []api.ToolInfo {
	tools := s.toolRegistry.ListByCategory(category)
	result := make([]api.ToolInfo, len(tools))
	for i, t := range tools {
		result[i] = api.ToolInfo{
			Name:         t.Name(),
			Category:     t.Category(),
			Description:  t.Description(),
			InputSchema:  t.InputSchema(),
		}
	}
	return result
}

func (s *BiolabMCPService) ExecuteTool(ctx context.Context, category, name string, input map[string]interface{}) (map[string]interface{}, error) {
	// Proxy to external biolab-mcp-server
	url := fmt.Sprintf("%s/api/v1/tools/%s/%s/execute", s.biolabMCPURL, category, name)
	
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Fallback to local mock tool if external service unavailable
		s.logger.Warn("External biolab-mcp-server unavailable, falling back to mock", zap.Error(err))
		tool, ok := s.toolRegistry.Get(category, name)
		if !ok {
			return nil, fmt.Errorf("tool not found: %s", name)
		}
		return tool.Execute(ctx, input)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("biolab-mcp-server error: %s - %s", resp.Status, string(body))
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

func (s *BiolabMCPService) GetToolSchema(category, name string) (map[string]interface{}, bool) {
	tool, ok := s.toolRegistry.Get(category, name)
	if !ok {
		return nil, false
	}
	return tool.InputSchema(), true
}

func (s *BiolabMCPService) CreateSandboxSession(ctx context.Context, experimentID string, metadata map[string]interface{}) (*api.SandboxSession, error) {
	session, err := s.sandbox.CreateSession(ctx, experimentID, metadata)
	if err != nil {
		return nil, err
	}
	return &api.SandboxSession{
		ID:           session.ID,
		ExperimentID: session.ExperimentID,
		Status:       session.Status,
		WorkDir:      session.WorkDir,
		Metadata:     session.Metadata,
		CreatedAt:    session.CreatedAt,
	}, nil
}

func (s *BiolabMCPService) ListSandboxSessions() []*api.SandboxSession {
	sessions := s.sandbox.ListSessions()
	result := make([]*api.SandboxSession, len(sessions))
	for i, s := range sessions {
		result[i] = &api.SandboxSession{
			ID:           s.ID,
			ExperimentID: s.ExperimentID,
			Status:       s.Status,
			WorkDir:      s.WorkDir,
			Metadata:     s.Metadata,
			CreatedAt:    s.CreatedAt,
		}
	}
	return result
}

func (s *BiolabMCPService) GetSandboxSession(id string) (*api.SandboxSession, bool) {
	session, ok := s.sandbox.GetSession(id)
	if !ok {
		return nil, false
	}
	return &api.SandboxSession{
		ID:           session.ID,
		ExperimentID: session.ExperimentID,
		Status:       session.Status,
		WorkDir:      session.WorkDir,
		Metadata:     session.Metadata,
		CreatedAt:    session.CreatedAt,
	}, true
}

func (s *BiolabMCPService) ExecuteSandboxExperiment(ctx context.Context, sessionID string, spec map[string]interface{}) (*api.SandboxSession, error) {
	var expSpec ExperimentSpec
	// Convert spec to ExperimentSpec
	// For now, just run the experiment
	session, err := s.sandbox.RunExperiment(ctx, sessionID, expSpec)
	if err != nil {
		return nil, err
	}
	return &api.SandboxSession{
		ID:           session.ID,
		ExperimentID: session.ExperimentID,
		Status:       session.Status,
		WorkDir:      session.WorkDir,
		Metadata:     session.Metadata,
		CreatedAt:    session.CreatedAt,
	}, nil
}

func (s *BiolabMCPService) SendNotification(ctx context.Context, notificationType string, recipients []string, channels []string, data map[string]interface{}) (map[string]interface{}, error) {
	notifierAgent, ok := s.orchestrator.GetAgent(AgentID("notifier"))
	if !ok {
		return nil, fmt.Errorf("notifier agent not found")
	}

	task := api.Task{
		ID:   uuid.New().String(),
		Type: "notify",
		Input: map[string]interface{}{
			"notification_type": notificationType,
			"recipients":        recipients,
			"channels":          channels,
			"data":              data,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := notifierAgent.Execute(ctx, task)
	if err != nil {
		return nil, err
	}
	return result.Output, nil
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1" || v == "yes"
	}
	return fallback
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func NewBiolabMCPServiceImpl(logger *zap.Logger, config *BiolabConfig) *BiolabMCPService {
	return NewBiolabMCPService(logger, config)
}