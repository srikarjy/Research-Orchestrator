package agents

import (
	"context"
	"encoding/json"
	"time"
)

type AgentID string

const (
	AgentPlanner          AgentID = "planner"
	AgentResearcher       AgentID = "researcher"
	AgentCritic           AgentID = "critic"
	AgentExecutor         AgentID = "executor"
	AgentValidator        AgentID = "validator"
	AgentNotifier         AgentID = "notifier"
	AgentClinicalTrial    AgentID = "clinical_trial"
	AgentRegulatory       AgentID = "regulatory"
	AgentBiomarker        AgentID = "biomarker"
)

type AgentStatus string

const (
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusWaiting   AgentStatus = "waiting"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusFailed    AgentStatus = "failed"
)

type MessageType string

const (
	MessageTypeTask       MessageType = "task"
	MessageTypeResult     MessageType = "result"
	MessageTypeRequest    MessageType = "request"
	MessageTypeResponse   MessageType = "response"
	MessageTypeNotification MessageType = "notification"
	MessageTypeError      MessageType = "error"
)

type Message struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	From      AgentID         `json:"from"`
	To        AgentID         `json:"to"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	TraceID   string          `json:"trace_id"`
}

type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Priority    int                    `json:"priority"`
	Dependencies []string              `json:"dependencies"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type Result struct {
	TaskID      string                 `json:"task_id"`
	AgentID     AgentID                `json:"agent_id"`
	Status      string                 `json:"status"`
	Output      map[string]interface{} `json:"output"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Artifacts   []Artifact             `json:"artifacts"`
}

type Artifact struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Content     interface{}            `json:"content"`
	Path        string                 `json:"path,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type AgentConfig struct {
	ID          AgentID                `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Capabilities []string              `json:"capabilities"`
	MaxRetries  int                    `json:"max_retries"`
	Timeout     time.Duration          `json:"timeout"`
	Settings    map[string]interface{} `json:"settings"`
}

type Agent interface {
	ID() AgentID
	Name() string
	Description() string
	Capabilities() []string
	Config() AgentConfig
	Execute(ctx context.Context, task Task) (Result, error)
	HandleMessage(ctx context.Context, msg Message) (Message, error)
	Status() AgentStatus
	SetStatus(status AgentStatus)
	HealthCheck(ctx context.Context) error
}