package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/srikarjy/research-orchestrator/assayos/internal/api"
	"go.uber.org/zap"
)

// StepStatus represents the status of a workflow step
type StepStatus string

const (
	StepStatusPending        StepStatus = "pending"
	StepStatusRunning        StepStatus = "running"
	StepStatusCompleted      StepStatus = "completed"
	StepStatusFailed         StepStatus = "failed"
	StepStatusAwaitingReview StepStatus = "awaiting_review"
)

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Category        string                 `json:"category"` // retriever, analyzer, visualizer, executor
	Tool            string                 `json:"tool"`
	Input           map[string]interface{} `json:"input"`
	Output          map[string]interface{} `json:"output,omitempty"`
	Status          StepStatus             `json:"status"`
	Retries         int                    `json:"retries"`
	MaxRetries      int                    `json:"max_retries"`
	StartedAt       *time.Time             `json:"started_at,omitempty"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	Error           string                 `json:"error,omitempty"`
	DependsOn       []string               `json:"depends_on,omitempty"`
	IdempotencyKey  string                 `json:"idempotency_key,omitempty"`
}

// Workflow represents a complete workflow
type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Query       string                 `json:"query"`
	Steps       []*WorkflowStep        `json:"steps"`
	Status      string                 `json:"status"` // running, completed, failed, awaiting_review
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowEvent represents an event in the workflow execution
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

// EventStore interface for event logging
type EventStore interface {
	Append(event *WorkflowEvent) error
	GetByWorkflowID(workflowID string) ([]*WorkflowEvent, error)
	GetByDedupKey(dedupKey string) (*WorkflowEvent, error)
	GetByIdempotencyKey(key string) (*WorkflowEvent, error)
	GetLatestEvent(workflowID string) (*WorkflowEvent, error)
	Close() error
}

// In-memory event store implementation
type MemoryEventStore struct {
	mu      sync.RWMutex
	events  map[string]*WorkflowEvent
	byWF    map[string][]*WorkflowEvent
	byDedup map[string]*WorkflowEvent
	byIdemp map[string]*WorkflowEvent
}

func NewMemoryEventStore() *MemoryEventStore {
	return &MemoryEventStore{
		events:  make(map[string]*WorkflowEvent),
		byWF:    make(map[string][]*WorkflowEvent),
		byDedup: make(map[string]*WorkflowEvent),
		byIdemp: make(map[string]*WorkflowEvent),
	}
}

func (s *MemoryEventStore) Append(event *WorkflowEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byDedup[event.DedupKey]; exists {
		return fmt.Errorf("duplicate dedup key: %s", event.DedupKey)
	}
	if event.IdempotencyKey != "" {
		if _, exists := s.byIdemp[event.IdempotencyKey]; exists {
			return fmt.Errorf("duplicate idempotency key: %s", event.IdempotencyKey)
		}
	}

	s.events[event.ID] = event
	s.byWF[event.WorkflowID] = append(s.byWF[event.WorkflowID], event)
	s.byDedup[event.DedupKey] = event
	if event.IdempotencyKey != "" {
		s.byIdemp[event.IdempotencyKey] = event
	}
	return nil
}

func (s *MemoryEventStore) GetByWorkflowID(workflowID string) ([]*WorkflowEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.byWF[workflowID]
	result := make([]*WorkflowEvent, len(events))
	copy(result, events)
	return result, nil
}

func (s *MemoryEventStore) GetByDedupKey(dedupKey string) (*WorkflowEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.byDedup[dedupKey]; ok {
		return e, nil
	}
	return nil, errors.New("event not found")
}

func (s *MemoryEventStore) GetByIdempotencyKey(key string) (*WorkflowEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.byIdemp[key]; ok {
		return e, nil
	}
	return nil, errors.New("event not found")
}

func (s *MemoryEventStore) GetLatestEvent(workflowID string) (*WorkflowEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.byWF[workflowID]
	if len(events) == 0 {
		return nil, errors.New("event not found")
	}
	return events[len(events)-1], nil
}

func (s *MemoryEventStore) Close() error {
	return nil
}

// StepExecutor interface for executing workflow steps
type StepExecutor interface {
	Execute(ctx context.Context, step *WorkflowStep) (map[string]interface{}, error)
	GetToolName() string
	GetCategory() string
}

// WorkflowEngine holds the workflow execution engine
type WorkflowEngine struct {
	store       EventStore
	executors   map[string]StepExecutor
	workflows   map[string]*Workflow
	eventCallback func(string, *WorkflowEvent)
	mu          sync.RWMutex
}

func NewWorkflowEngine(store EventStore) *WorkflowEngine {
	return &WorkflowEngine{
		store:     store,
		executors: make(map[string]StepExecutor),
		workflows: make(map[string]*Workflow),
	}
}

func (e *WorkflowEngine) SetEventCallback(callback func(string, *WorkflowEvent)) {
	e.eventCallback = callback
}

func (e *WorkflowEngine) RegisterExecutor(executor StepExecutor) {
	key := executor.GetCategory() + ":" + executor.GetToolName()
	e.executors[key] = executor
}

func (e *WorkflowEngine) CreateWorkflow(ctx context.Context, name, query string, steps []*WorkflowStep) (*Workflow, error) {
	wf := &Workflow{
		ID:        uuid.New().String(),
		Name:      name,
		Query:     query,
		Steps:     steps,
		Status:    "running",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Metadata:  make(map[string]interface{}),
	}

	// Generate idempotency keys for each step
	for _, step := range wf.Steps {
		if step.IdempotencyKey == "" {
			step.IdempotencyKey = e.generateStepIdempotencyKey(wf.ID, step.ID)
		}
	}

	// Persist workflow start event
	event := &WorkflowEvent{
		ID:          uuid.New().String(),
		WorkflowID:  wf.ID,
		Type:        "workflow_started",
		Timestamp:   time.Now().UTC(),
		Payload:     map[string]interface{}{"query": query, "name": name},
		DedupKey:    e.generateDedupKey(wf.ID, "", "workflow_started", map[string]interface{}{"query": query, "name": name}),
		IdempotencyKey: "",
	}
	if err := e.appendEvent(event); err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.workflows[wf.ID] = wf
	e.mu.Unlock()
	return wf, nil
}

func (e *WorkflowEngine) appendEvent(event *WorkflowEvent) error {
	if err := e.store.Append(event); err != nil {
		return err
	}
	if e.eventCallback != nil {
		e.eventCallback(event.WorkflowID, event)
	}
	return nil
}

func (e *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflowID string) error {
	e.mu.RLock()
	wf, ok := e.workflows[workflowID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	executed := make(map[string]bool)

	for {
		progress := false
		for _, step := range wf.Steps {
			if executed[step.ID] {
				continue
			}

			depsMet := true
			for _, dep := range step.DependsOn {
				if !executed[dep] {
					depsMet = false
					break
				}
			}
			if !depsMet {
				continue
			}

			if err := e.executeStep(ctx, wf, step); err != nil {
				return err
			}
			executed[step.ID] = true
			progress = true
		}

		if !progress {
			break
		}
	}

	now := time.Now().UTC()
	wf.Status = "completed"
	wf.CompletedAt = &now
	wf.UpdatedAt = now

	event := &WorkflowEvent{
		ID:          uuid.New().String(),
		WorkflowID:  wf.ID,
		Type:        "workflow_completed",
		Timestamp:   now,
		Payload:     map[string]interface{}{"status": "completed"},
		DedupKey:    e.generateDedupKey(wf.ID, "", "workflow_completed", map[string]interface{}{"status": "completed"}),
		IdempotencyKey: "",
	}
	return e.appendEvent(event)
}

func (e *WorkflowEngine) executeStep(ctx context.Context, wf *Workflow, step *WorkflowStep) error {
	executorKey := step.Category + ":" + step.Tool
	executor, ok := e.executors[executorKey]
	if !ok {
		return e.completeStep(ctx, wf, step, map[string]interface{}{"simulated": true}, nil)
	}

	if step.IdempotencyKey != "" {
		existing, err := e.store.GetByIdempotencyKey(step.IdempotencyKey)
		if err == nil && existing != nil {
			if output, ok := existing.Payload["output"].(map[string]interface{}); ok {
				step.Output = output
				step.Status = StepStatusCompleted
				return nil
			}
		}
	}

	startEvent := &WorkflowEvent{
		ID:          uuid.New().String(),
		WorkflowID:  wf.ID,
		StepID:      step.ID,
		Type:        "step_started",
		Timestamp:   time.Now().UTC(),
		Payload:     map[string]interface{}{"tool": step.Tool, "input": step.Input},
		DedupKey:    e.generateDedupKey(wf.ID, step.ID, "step_started", map[string]interface{}{"tool": step.Tool, "input": step.Input}),
		IdempotencyKey: step.IdempotencyKey,
	}
	if err := e.appendEvent(startEvent); err != nil {
		return err
	}

	now := time.Now().UTC()
	step.Status = StepStatusRunning
	step.StartedAt = &now
	wf.UpdatedAt = now

	output, err := executor.Execute(ctx, step)

	if err != nil {
		step.Retries++
		if step.Retries > step.MaxRetries {
			step.Status = StepStatusFailed
			step.Error = err.Error()
			failEvent := &WorkflowEvent{
				ID:          uuid.New().String(),
				WorkflowID:  wf.ID,
				StepID:      step.ID,
				Type:        "step_failed",
				Timestamp:   time.Now().UTC(),
				Payload:     map[string]interface{}{"tool": step.Tool, "error": err.Error(), "retries": step.Retries},
				DedupKey:    e.generateDedupKey(wf.ID, step.ID, "step_failed", map[string]interface{}{"tool": step.Tool, "error": err.Error(), "retries": step.Retries}),
				IdempotencyKey: step.IdempotencyKey,
			}
			e.appendEvent(failEvent)
			return fmt.Errorf("max retries exceeded")
		}

		retryEvent := &WorkflowEvent{
			ID:          uuid.New().String(),
			WorkflowID:  wf.ID,
			StepID:      step.ID,
			Type:        "step_retried",
			Timestamp:   time.Now().UTC(),
			Payload:     map[string]interface{}{"tool": step.Tool, "attempt": step.Retries, "error": err.Error()},
			DedupKey:    e.generateDedupKey(wf.ID, step.ID, "step_retried", map[string]interface{}{"tool": step.Tool, "attempt": step.Retries, "error": err.Error()}),
			IdempotencyKey: step.IdempotencyKey,
		}
		e.appendEvent(retryEvent)
		return e.executeStep(ctx, wf, step)
	}

	return e.completeStep(ctx, wf, step, output, nil)
}

func (e *WorkflowEngine) completeStep(ctx context.Context, wf *Workflow, step *WorkflowStep, output map[string]interface{}, err error) error {
	now := time.Now().UTC()
	step.Status = StepStatusCompleted
	step.CompletedAt = &now
	step.Output = output
	wf.UpdatedAt = now

	latencyMs := 0
	if step.StartedAt != nil {
		latencyMs = int(now.Sub(*step.StartedAt).Milliseconds())
	}

	completeEvent := &WorkflowEvent{
		ID:          uuid.New().String(),
		WorkflowID:  wf.ID,
		StepID:      step.ID,
		Type:        "step_completed",
		Timestamp:   now,
		Payload:     map[string]interface{}{"tool": step.Tool, "output": output, "latency_ms": latencyMs, "retries": step.Retries},
		DedupKey:    e.generateDedupKey(wf.ID, step.ID, "step_completed", map[string]interface{}{"tool": step.Tool, "output": output, "latency_ms": latencyMs, "retries": step.Retries}),
		IdempotencyKey: step.IdempotencyKey,
	}

	return e.appendEvent(completeEvent)
}

func (e *WorkflowEngine) GetWorkflow(workflowID string) (*Workflow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	wf, ok := e.workflows[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	return wf, nil
}

func (e *WorkflowEngine) GetAllWorkflows() map[string]*Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.workflows
}

func (e *WorkflowEngine) GetWorkflowEvents(workflowID string) ([]*WorkflowEvent, error) {
	return e.store.GetByWorkflowID(workflowID)
}

func (e *WorkflowEngine) generateDedupKey(workflowID, stepID string, typ string, payload map[string]interface{}) string {
	// Simple hash - in production use sha256
	return fmt.Sprintf("%s-%s-%s", workflowID, stepID, typ)
}

func (e *WorkflowEngine) generateStepIdempotencyKey(workflowID, stepID string) string {
	return "idem-" + workflowID + "-" + stepID
}

func BuildDemoWorkflow(query string) (*Workflow, []*WorkflowStep) {
	steps := []*WorkflowStep{
		{ID: "pubmed", Name: "PubMed Search", Category: "retriever", Tool: "PubMed", Input: map[string]interface{}{"query": query}, MaxRetries: 3},
		{ID: "uniprot", Name: "UniProt Lookup", Category: "retriever", Tool: "UniProt", Input: map[string]interface{}{"query": query}, MaxRetries: 3},
		{ID: "chembl", Name: "ChEMBL Search", Category: "retriever", Tool: "ChEMBL", Input: map[string]interface{}{"query": query}, MaxRetries: 3},
		{ID: "stability", Name: "Protein Stability Prediction", Category: "analyzer", Tool: "ProteinStabilityPredictor", Input: map[string]interface{}{"mutation": "V600E"}, MaxRetries: 2, DependsOn: []string{"uniprot"}},
		{ID: "docking", Name: "Molecular Docking", Category: "analyzer", Tool: "Docking", Input: map[string]interface{}{"ligand": "vemurafenib"}, MaxRetries: 2, DependsOn: []string{"stability", "chembl"}},
		{ID: "merge", Name: "Evidence Merge", Category: "analyzer", Tool: "EvidenceMerge", Input: map[string]interface{}{}, MaxRetries: 1, DependsOn: []string{"pubmed", "docking"}},
		{ID: "critic", Name: "Critic Agent", Category: "analyzer", Tool: "Critic", Input: map[string]interface{}{}, MaxRetries: 1, DependsOn: []string{"merge"}},
		{ID: "structure_view", Name: "Structure Viewer", Category: "visualizer", Tool: "StructureViewer", Input: map[string]interface{}{"pdb_id": "4RZW"}, MaxRetries: 1, DependsOn: []string{"critic"}},
	}
	return nil, steps
}

// MockExecutor for testing
type MockExecutor struct {
	ToolName string
	Category string
}

func (m *MockExecutor) Execute(ctx context.Context, step *WorkflowStep) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool":      m.ToolName,
		"status":    "completed",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"simulated": true,
	}, nil
}

func (m *MockExecutor) GetToolName() string { return m.ToolName }
func (m *MockExecutor) GetCategory() string { return m.Category }

// WorkflowEngineService wraps the engine for the gateway
type WorkflowEngineService struct {
	logger  *zap.Logger
	engine  *WorkflowEngine
	store   EventStore
	mu      sync.RWMutex
}

func NewWorkflowEngineService(logger *zap.Logger) *WorkflowEngineService {
	logger = logger.Named("workflow-engine")
	memStore := NewMemoryEventStore()
	eng := NewWorkflowEngine(memStore)

	eng.RegisterExecutor(&MockExecutor{ToolName: "PubMed", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "UniProt", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "ChEMBL", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "ProteinStabilityPredictor", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "Docking", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "EvidenceMerge", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "Critic", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "StructureViewer", Category: "visualizer"})

	return &WorkflowEngineService{
		logger: logger,
		engine: eng,
		store:  memStore,
	}
}

func (s *WorkflowEngineService) Name() string { return "workflow-engine" }

func (s *WorkflowEngineService) Start(ctx context.Context) error {
	s.logger.Info("Workflow Engine service started")
	return nil
}

func (s *WorkflowEngineService) Stop(ctx context.Context) error {
	s.logger.Info("Workflow Engine service stopped")
	return s.store.Close()
}

func (s *WorkflowEngineService) Health(ctx context.Context) error {
	return nil
}

func (s *WorkflowEngineService) CreateWorkflow(ctx context.Context, name, query string, steps []*api.WorkflowStep) (*api.Workflow, error) {
	// Convert api.WorkflowStep to local WorkflowStep
	localSteps := make([]*WorkflowStep, len(steps))
	for i, step := range steps {
		localSteps[i] = &WorkflowStep{
			ID:              step.ID,
			Name:            step.Name,
			Category:        step.Category,
			Tool:            step.Tool,
			Input:           step.Input,
			Output:          step.Output,
			Status:          StepStatus(step.Status),
			Retries:         step.Retries,
			MaxRetries:      step.MaxRetries,
			StartedAt:       step.StartedAt,
			CompletedAt:     step.CompletedAt,
			Error:           step.Error,
			DependsOn:       step.DependsOn,
			IdempotencyKey:  step.IdempotencyKey,
		}
	}
	wf, err := s.engine.CreateWorkflow(ctx, name, query, localSteps)
	if err != nil {
		return nil, err
	}
	return s.toAPIWorkflow(wf), nil
}

func (s *WorkflowEngineService) ExecuteWorkflow(ctx context.Context, id string) error {
	return s.engine.ExecuteWorkflow(ctx, id)
}

func (s *WorkflowEngineService) GetWorkflow(id string) (*api.Workflow, error) {
	wf, err := s.engine.GetWorkflow(id)
	if err != nil {
		return nil, err
	}
	return s.toAPIWorkflow(wf), nil
}

func (s *WorkflowEngineService) ListWorkflows() []*api.Workflow {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workflows := s.engine.GetAllWorkflows()
	result := make([]*api.Workflow, 0, len(workflows))
	for _, wf := range workflows {
		result = append(result, s.toAPIWorkflow(wf))
	}
	return result
}

func (s *WorkflowEngineService) GetEvents(id string) ([]*api.WorkflowEvent, error) {
	events, err := s.engine.GetWorkflowEvents(id)
	if err != nil {
		return nil, err
	}
	result := make([]*api.WorkflowEvent, len(events))
	for i, e := range events {
		result[i] = s.toAPIEvent(e)
	}
	return result, nil
}

func (s *WorkflowEngineService) GetStep(workflowID, stepID string) (*api.WorkflowStep, error) {
	wf, err := s.engine.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}

	for _, step := range wf.Steps {
		if step.ID == stepID {
			return s.toAPIStep(step), nil
		}
	}
	return nil, fmt.Errorf("step not found: %s", stepID)
}

func (s *WorkflowEngineService) toAPIWorkflow(wf *Workflow) *api.Workflow {
	steps := make([]*api.WorkflowStep, len(wf.Steps))
	for i, step := range wf.Steps {
		steps[i] = s.toAPIStep(step)
	}

	return &api.Workflow{
		ID:          wf.ID,
		Name:        wf.Name,
		Query:       wf.Query,
		Steps:       steps,
		Status:      wf.Status,
		CreatedAt:   wf.CreatedAt,
		UpdatedAt:   wf.UpdatedAt,
		CompletedAt: wf.CompletedAt,
		Metadata:    wf.Metadata,
	}
}

func (s *WorkflowEngineService) toAPIStep(step *WorkflowStep) *api.WorkflowStep {
	return &api.WorkflowStep{
		ID:              step.ID,
		Name:            step.Name,
		Category:        step.Category,
		Tool:            step.Tool,
		Input:           step.Input,
		Output:          step.Output,
		Status:          string(step.Status),
		Retries:         step.Retries,
		MaxRetries:      step.MaxRetries,
		StartedAt:       step.StartedAt,
		CompletedAt:     step.CompletedAt,
		Error:           step.Error,
		DependsOn:       step.DependsOn,
		IdempotencyKey:  step.IdempotencyKey,
	}
}

func (s *WorkflowEngineService) toAPIEvent(e *WorkflowEvent) *api.WorkflowEvent {
	return &api.WorkflowEvent{
		ID:              e.ID,
		WorkflowID:      e.WorkflowID,
		StepID:          e.StepID,
		Type:            e.Type,
		Source:          e.Source,
		TraceID:         e.TraceID,
		Payload:         e.Payload,
		DedupKey:        e.DedupKey,
		IdempotencyKey:  e.IdempotencyKey,
		Timestamp:       e.Timestamp,
	}
}