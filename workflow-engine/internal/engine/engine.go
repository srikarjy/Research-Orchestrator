package engine

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/srikarjy/research-orchestrator/workflow-engine/pkg/eventlog"
)

var (
	ErrStepNotFound      = errors.New("step not found")
	ErrWorkflowNotFound  = errors.New("workflow not found")
	ErrAlreadyCompleted  = errors.New("step already completed")
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

type StepStatus string

const (
	StepStatusPending    StepStatus = "pending"
	StepStatusRunning    StepStatus = "running"
	StepStatusCompleted  StepStatus = "completed"
	StepStatusFailed     StepStatus = "failed"
	StepStatusAwaitingReview StepStatus = "awaiting_review"
)

type Step struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Category    string                 `json:"category"` // retriever, analyzer, visualizer, executor
	Tool        string                 `json:"tool"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Status      StepStatus             `json:"status"`
	Retries     int                    `json:"retries"`
	MaxRetries  int                    `json:"max_retries"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	IdempotencyKey string              `json:"idempotency_key,omitempty"`
}

type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Query       string                 `json:"query"`
	Steps       []*Step                `json:"steps"`
	Status      string                 `json:"status"` // running, completed, failed, awaiting_review
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type StepExecutor interface {
	Execute(ctx context.Context, step *Step) (map[string]interface{}, error)
	GetToolName() string
	GetCategory() string
}

type Engine struct {
	store     eventlog.Store
	executors map[string]StepExecutor
	workflows map[string]*Workflow
}

func NewEngine(store eventlog.Store) *Engine {
	return &Engine{
		store:     store,
		executors: make(map[string]StepExecutor),
		workflows: make(map[string]*Workflow),
	}
}

func (e *Engine) RegisterExecutor(executor StepExecutor) {
	key := executor.GetCategory() + ":" + executor.GetToolName()
	e.executors[key] = executor
}

func (e *Engine) CreateWorkflow(ctx context.Context, name, query string, steps []*Step) (*Workflow, error) {
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
			step.IdempotencyKey = GenerateStepIdempotencyKey(wf.ID, step.ID)
		}
	}

	// Persist workflow start event
	event := eventlog.NewEvent(wf.ID, "", eventlog.EventTypeWorkflowStarted, map[string]interface{}{
		"query": query,
		"name":  name,
	}, "")
	if err := e.store.Append(event); err != nil {
		return nil, err
	}

	e.workflows[wf.ID] = wf
	return wf, nil
}

func (e *Engine) ExecuteWorkflow(ctx context.Context, workflowID string) error {
	wf, ok := e.workflows[workflowID]
	if !ok {
		return ErrWorkflowNotFound
	}

	// Build dependency graph and execute in topological order
	executed := make(map[string]bool)
	
	for {
		progress := false
		for _, step := range wf.Steps {
			if executed[step.ID] {
				continue
			}
			
			// Check dependencies
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

	// Mark workflow completed
	now := time.Now().UTC()
	wf.Status = "completed"
	wf.CompletedAt = &now
	wf.UpdatedAt = now

	event := eventlog.NewEvent(wf.ID, "", eventlog.EventTypeWorkflowCompleted, map[string]interface{}{
		"status": "completed",
	}, "")
	return e.store.Append(event)
}

func (e *Engine) executeStep(ctx context.Context, wf *Workflow, step *Step) error {
	executorKey := step.Category + ":" + step.Tool
	executor, ok := e.executors[executorKey]
	if !ok {
		// No executor registered - simulate completion for demo
		return e.completeStep(ctx, wf, step, map[string]interface{}{"simulated": true}, nil)
	}

	// Check idempotency - has this step already completed?
	if step.IdempotencyKey != "" {
		existing, err := e.store.GetByIdempotencyKey(step.IdempotencyKey)
		if err == nil && existing != nil {
			// Step already executed, restore output
			if output, ok := existing.Payload["output"].(map[string]interface{}); ok {
				step.Output = output
				step.Status = StepStatusCompleted
				return nil
			}
		}
	}

	// Emit step started event
	startEvent := eventlog.NewEvent(wf.ID, step.ID, eventlog.EventTypeStepStarted, map[string]interface{}{
		"tool": step.Tool,
		"input": step.Input,
	}, step.IdempotencyKey)
	if err := e.store.Append(startEvent); err != nil {
		if err == eventlog.ErrDuplicateDedupKey {
			// Already started, check if completed
			return e.checkStepCompletion(ctx, wf, step)
		}
		return err
	}

	now := time.Now().UTC()
	step.Status = StepStatusRunning
	step.StartedAt = &now
	wf.UpdatedAt = now

	// Execute the step
	output, err := executor.Execute(ctx, step)
	
	if err != nil {
		step.Retries++
		if step.Retries > step.MaxRetries {
			step.Status = StepStatusFailed
			step.Error = err.Error()
			failEvent := eventlog.NewEvent(wf.ID, step.ID, eventlog.EventTypeStepFailed, map[string]interface{}{
				"tool": step.Tool,
				"error": err.Error(),
				"retries": step.Retries,
			}, step.IdempotencyKey)
			e.store.Append(failEvent)
			return ErrMaxRetriesExceeded
		}
		
		retryEvent := eventlog.NewEvent(wf.ID, step.ID, eventlog.EventTypeStepRetried, map[string]interface{}{
			"tool": step.Tool,
			"attempt": step.Retries,
			"error": err.Error(),
		}, step.IdempotencyKey)
		e.store.Append(retryEvent)
		return e.executeStep(ctx, wf, step) // Retry
	}

	return e.completeStep(ctx, wf, step, output, nil)
}

func (e *Engine) completeStep(ctx context.Context, wf *Workflow, step *Step, output map[string]interface{}, err error) error {
	now := time.Now().UTC()
	step.Status = StepStatusCompleted
	step.CompletedAt = &now
	step.Output = output
	wf.UpdatedAt = now

	latencyMs := 0
	if step.StartedAt != nil {
		latencyMs = int(now.Sub(*step.StartedAt).Milliseconds())
	}

	completeEvent := eventlog.NewEvent(wf.ID, step.ID, eventlog.EventTypeStepCompleted, map[string]interface{}{
		"tool": step.Tool,
		"output": output,
		"latency_ms": latencyMs,
		"retries": step.Retries,
	}, step.IdempotencyKey)
	
	return e.store.Append(completeEvent)
}

func (e *Engine) checkStepCompletion(ctx context.Context, wf *Workflow, step *Step) error {
	events, err := e.store.GetByWorkflowID(wf.ID)
	if err != nil {
		return err
	}
	for _, evt := range events {
		if evt.StepID == step.ID && evt.Type == eventlog.EventTypeStepCompleted {
			if output, ok := evt.Payload["output"].(map[string]interface{}); ok {
				step.Output = output
				step.Status = StepStatusCompleted
				return nil
			}
		}
	}
	return nil
}

func (e *Engine) GetWorkflow(workflowID string) (*Workflow, error) {
	wf, ok := e.workflows[workflowID]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	return wf, nil
}

func (e *Engine) GetAllWorkflows() map[string]*Workflow {
	return e.workflows
}

func (e *Engine) GetWorkflowEvents(workflowID string) ([]*eventlog.Event, error) {
	return e.store.GetByWorkflowID(workflowID)
}

func GenerateStepIdempotencyKey(workflowID, stepID string) string {
	return "idem-" + workflowID + "-" + stepID
}

func BuildDemoWorkflow(query string) (*Workflow, []*Step) {
	steps := []*Step{
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