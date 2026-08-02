package agents

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAgentNotFound   = errors.New("agent not found")
	ErrAgentBusy       = errors.New("agent busy")
	ErrAgentTimeout    = errors.New("agent timeout")
	ErrInvalidTask     = errors.New("invalid task")
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

type BaseAgent struct {
	config   AgentConfig
	status   AgentStatus
	mu       sync.RWMutex
	msgBus   MessageBus
}

func NewBaseAgent(config AgentConfig, msgBus MessageBus) *BaseAgent {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Minute
	}
	return &BaseAgent{
		config: config,
		status: AgentStatusIdle,
		msgBus: msgBus,
	}
}

func (a *BaseAgent) ID() AgentID                 { return a.config.ID }
func (a *BaseAgent) Name() string                { return a.config.Name }
func (a *BaseAgent) Description() string         { return a.config.Description }
func (a *BaseAgent) Capabilities() []string      { return a.config.Capabilities }
func (a *BaseAgent) Config() AgentConfig         { return a.config }

func (a *BaseAgent) Status() AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

func (a *BaseAgent) SetStatus(status AgentStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
}

func (a *BaseAgent) Execute(ctx context.Context, task Task) (Result, error) {
	return Result{}, ErrInvalidTask
}

func (a *BaseAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	return Message{}, nil
}

func (a *BaseAgent) HealthCheck(ctx context.Context) error {
	return nil
}

func (a *BaseAgent) sendMessage(ctx context.Context, to AgentID, msgType MessageType, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := Message{
		ID:        uuid.New().String(),
		Type:      msgType,
		From:      a.config.ID,
		To:        to,
		Payload:   data,
		Timestamp: time.Now(),
		TraceID:   uuid.New().String(),
	}
	return a.msgBus.Publish(ctx, msg)
}

func (a *BaseAgent) sendResult(ctx context.Context, to AgentID, taskID string, output map[string]interface{}, err error) error {
	result := Result{
		TaskID:  taskID,
		AgentID: a.config.ID,
		Status:  "completed",
		Output:  output,
	}
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
	}
	return a.sendMessage(ctx, to, MessageTypeResult, result)
}

func (a *BaseAgent) withRetry(ctx context.Context, task Task, fn func(context.Context) (Result, error)) (Result, error) {
	var lastErr error
	for attempt := 0; attempt <= a.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return Result{}, lastErr
}