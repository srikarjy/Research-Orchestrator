package api

import (
	"context"
	"time"
)

type WorkflowEngineService interface {
	CreateWorkflow(ctx context.Context, name, query string, steps []*WorkflowStep) (*Workflow, error)
	GetWorkflow(id string) (*Workflow, error)
	ListWorkflows() []*Workflow
	ExecuteWorkflow(ctx context.Context, id string) error
	GetEvents(id string) ([]*WorkflowEvent, error)
	GetStep(workflowID, stepID string) (*WorkflowStep, error)
	Health(ctx context.Context) error
	// Embed kernel.Service interface
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type BiolabMCPService interface {
	ListAgents() []AgentInfo
	GetAgentStatus(id string) (AgentStatus, bool)
	CreateWorkflow(name, description string, tasks []Task) *BiolabWorkflow
	GetWorkflow(id string) (*BiolabWorkflow, bool)
	ListWorkflows() []*BiolabWorkflow
	ExecuteWorkflow(ctx context.Context, id string) error
	DeleteWorkflow(id string) error
	ListTools(category string) []ToolInfo
	ExecuteTool(ctx context.Context, category, name string, input map[string]interface{}) (map[string]interface{}, error)
	GetToolSchema(category, name string) (map[string]interface{}, bool)
	CreateSandboxSession(ctx context.Context, experimentID string, metadata map[string]interface{}) (*SandboxSession, error)
	ListSandboxSessions() []*SandboxSession
	GetSandboxSession(id string) (*SandboxSession, bool)
	ExecuteSandboxExperiment(ctx context.Context, sessionID string, spec map[string]interface{}) (*SandboxSession, error)
	SendNotification(ctx context.Context, notificationType string, recipients []string, channels []string, data map[string]interface{}) (map[string]interface{}, error)
	Health(ctx context.Context) error
	// Embed kernel.Service interface
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) error
}

type WorkflowStep struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Category       string                 `json:"category"`
	Tool           string                 `json:"tool"`
	Input          map[string]interface{} `json:"input"`
	Output         map[string]interface{} `json:"output,omitempty"`
	Status         string                 `json:"status"`
	Retries        int                    `json:"retries"`
	MaxRetries     int                    `json:"max_retries"`
	StartedAt      *time.Time             `json:"started_at,omitempty"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
	Error          string                 `json:"error,omitempty"`
	DependsOn      []string               `json:"depends_on,omitempty"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
}

type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Query       string         `json:"query"`
	Steps       []*WorkflowStep `json:"steps"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type WorkflowEvent struct {
	ID              string                 `json:"id"`
	WorkflowID      string                 `json:"workflow_id"`
	StepID          string                 `json:"step_id"`
	Type            string                 `json:"type"`
	Source          string                 `json:"source"`
	TraceID         string                 `json:"trace_id"`
	Payload         map[string]interface{} `json:"payload"`
	DedupKey        string                 `json:"dedup_key"`
	IdempotencyKey  string                 `json:"idempotency_key,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

type AgentInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
}

type AgentStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Task struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Description  string                 `json:"description"`
	Input        map[string]interface{} `json:"input"`
	Priority     int                    `json:"priority"`
	Dependencies []string               `json:"dependencies"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type BiolabWorkflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Tasks       []Task                 `json:"tasks"`
	Status      string                 `json:"status"`
	Results     map[string]TaskResult  `json:"results"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

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

type ToolInfo struct {
	Name         string                 `json:"name"`
	Category     string                 `json:"category"`
	Description  string                 `json:"description"`
	InputSchema  map[string]interface{} `json:"input_schema"`
}

type SandboxSession struct {
	ID           string                 `json:"id"`
	ExperimentID string                 `json:"experiment_id"`
	Status       string                 `json:"status"`
	WorkDir      string                 `json:"work_dir"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
}