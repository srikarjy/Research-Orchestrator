package agents

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrWorkflowRunning  = errors.New("workflow already running")
)

type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
)

type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Tasks       []Task                 `json:"tasks"`
	Status      WorkflowStatus         `json:"status"`
	Results     map[string]Result      `json:"results"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type Orchestrator struct {
	mu           sync.RWMutex
	agents       map[AgentID]Agent
	workflows    map[string]*Workflow
	msgBus       MessageBus
	logger       *zap.Logger
	taskQueue    chan Task
	resultQueue  chan Result
	running      bool
	wg           sync.WaitGroup
}

func NewOrchestrator(msgBus MessageBus, logger *zap.Logger) *Orchestrator {
	return &Orchestrator{
		agents:      make(map[AgentID]Agent),
		workflows:   make(map[string]*Workflow),
		msgBus:      msgBus,
		logger:      logger,
		taskQueue:   make(chan Task, 100),
		resultQueue: make(chan Result, 100),
	}
}

func (o *Orchestrator) RegisterAgent(agent Agent) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	if _, exists := o.agents[agent.ID()]; exists {
		return fmt.Errorf("agent %s already registered", agent.ID())
	}
	
	o.agents[agent.ID()] = agent
	o.msgBus.Subscribe(agent.ID(), agent.HandleMessage)
	o.logger.Info("Agent registered", zap.String("agent", string(agent.ID())), zap.String("name", agent.Name()))
	return nil
}

func (o *Orchestrator) UnregisterAgent(id AgentID) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	_, ok := o.agents[id]
	if !ok {
		return ErrAgentNotFound
	}
	
	o.msgBus.Unsubscribe(id)
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

func (o *Orchestrator) CreateWorkflow(name, description string, tasks []Task) *Workflow {
	workflow := &Workflow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Tasks:       tasks,
		Status:      WorkflowStatusPending,
		Results:     make(map[string]Result),
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}
	
	o.mu.Lock()
	o.workflows[workflow.ID] = workflow
	o.mu.Unlock()
	
	return workflow
}

func (o *Orchestrator) GetWorkflow(id string) (*Workflow, bool) {
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
		return ErrWorkflowNotFound
	}
	if workflow.Status == WorkflowStatusRunning {
		o.mu.Unlock()
		return ErrWorkflowRunning
	}
	workflow.Status = WorkflowStatusRunning
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
			go func(t Task, a Agent) {
				ctx, cancel := context.WithTimeout(ctx, a.Config().Timeout)
				defer cancel()

				result, err := a.Execute(ctx, t)
				
				mu.Lock()
				completed[t.ID] = true
				workflow.Results[t.ID] = result
				mu.Unlock()

				if err != nil {
					o.logger.Error("Task failed", zap.String("task", t.ID), zap.Error(err))
				} else {
					o.logger.Info("Task completed", zap.String("task", t.ID), zap.String("agent", string(a.ID())))
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

func (o *Orchestrator) findAgentForTask(task Task) Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	for _, agent := range o.agents {
		for _, cap := range agent.Capabilities() {
			if cap == task.Type {
				if agent.Status() == AgentStatusIdle {
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
	
	workflow.Status = WorkflowStatusCompleted
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
	
	workflow.Status = WorkflowStatusFailed
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
		return ErrWorkflowNotFound
	}
	
	if workflow.Status != WorkflowStatusRunning && workflow.Status != WorkflowStatusPending {
		return errors.New("workflow not running")
	}
	
	workflow.Status = WorkflowStatusCancelled
	now := time.Now()
	workflow.CompletedAt = &now
	
	return nil
}

func (o *Orchestrator) ListWorkflows() []*Workflow {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	workflows := make([]*Workflow, 0, len(o.workflows))
	for _, w := range o.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

func (o *Orchestrator) GetAgentStatus() map[AgentID]AgentStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	status := make(map[AgentID]AgentStatus)
	for id, agent := range o.agents {
		status[id] = agent.Status()
	}
	return status
}