package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/srikarjy/research-orchestrator/orchestrator/internal/api"
	"github.com/srikarjy/research-orchestrator/orchestrator/internal/wfengine"
)

// workflowDef is what CreateWorkflow was given: the step list, cached
// in-process. workflow-Engine's Postgres schema is a pure append-only event
// log (workflows + events) — it never stores step *definitions*
// (name/category/tool/depends_on), only what happened to a step name once
// dispatched. So this cache is how GetWorkflow/GetStep can still render step
// metadata alongside live execution status.
//
// KNOWN LIMITATION: this cache is in-process only. If orchestrator
// restarts, a workflow created before the restart loses its renderable step
// list here even though workflow-Engine's own event log (the durable,
// authoritative execution record) is untouched and still queryable via
// ReplayEvents. Fixing this durably means either an orchestrator-owned
// Postgres table for step definitions or extending workflow-Engine's
// schema — intentionally not done in this pass.
type workflowDef struct {
	name  string
	query string
	steps []*api.WorkflowStep
}

// WorkflowEngineService implements api.WorkflowEngineService against the
// real standalone github.com/srikarjy/workflow-Engine service, via its
// Postgres event log and the Redis Stream its worker pool consumes — see
// internal/wfengine's package doc for why this is a wire-contract client
// rather than a Go package import.
//
// DependsOn on WorkflowStep is preserved on the type for API/frontend
// compatibility but does NOT drive execution order here: the real
// workflow-Engine's queue model has no built-in dependency resolution
// (each StepMessage is dispatched and processed independently), and adding
// a DAG scheduler on top of it was explicitly decided against — the target
// pipelines are naturally sequential. ExecuteWorkflow produces steps in the
// order they appear in the slice.
type WorkflowEngineService struct {
	logger *zap.Logger
	dsn    string
	queue  *wfengine.Queue
	store  *wfengine.Store

	mu   sync.RWMutex
	defs map[string]*workflowDef
}

// NewWorkflowEngineService constructs the service. rdb is orchestrator's
// own Redis client (shared with workflow-Engine's worker once the two
// docker-compose files point at one Redis instance — see the milestone
// tracked separately). dsn is workflow-Engine's own Postgres DSN, opened in
// Start rather than here so construction can't fail before the service
// lifecycle begins.
func NewWorkflowEngineService(logger *zap.Logger, dsn string, rdb *redis.Client, stream string) *WorkflowEngineService {
	return &WorkflowEngineService{
		logger: logger.Named("workflow-engine"),
		dsn:    dsn,
		queue:  wfengine.NewQueue(rdb, stream),
		defs:   make(map[string]*workflowDef),
	}
}

func (s *WorkflowEngineService) Name() string { return "workflow-engine" }

func (s *WorkflowEngineService) Start(ctx context.Context) error {
	store, err := wfengine.NewStore(ctx, s.dsn)
	if err != nil {
		return fmt.Errorf("workflow-engine: start: %w", err)
	}
	s.store = store
	s.logger.Info("Workflow Engine client started (wired to standalone workflow-Engine)")
	return nil
}

func (s *WorkflowEngineService) Stop(ctx context.Context) error {
	if s.store != nil {
		s.store.Close()
	}
	s.logger.Info("Workflow Engine client stopped")
	return nil
}

func (s *WorkflowEngineService) Health(ctx context.Context) error {
	if s.store == nil {
		return fmt.Errorf("workflow-engine: not started")
	}
	_, err := s.store.ListWorkflows(ctx, 1)
	return err
}

func (s *WorkflowEngineService) CreateWorkflow(ctx context.Context, name, query string, steps []*api.WorkflowStep) (*api.Workflow, error) {
	id := uuid.New()

	inputJSON, err := json.Marshal(map[string]any{"query": query})
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	if err := s.store.CreateWorkflow(ctx, id, name, inputJSON); err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}

	for _, step := range steps {
		if step.Status == "" {
			step.Status = "pending"
		}
	}

	s.mu.Lock()
	s.defs[id.String()] = &workflowDef{name: name, query: query, steps: steps}
	s.mu.Unlock()

	wf, err := s.store.GetWorkflow(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("read back created workflow: %w", err)
	}

	return &api.Workflow{
		ID:        id.String(),
		Name:      name,
		Query:     query,
		Steps:     steps,
		Status:    wf.Status,
		CreatedAt: wf.CreatedAt,
		UpdatedAt: wf.UpdatedAt,
		Metadata:  map[string]interface{}{},
	}, nil
}

// ExecuteWorkflow produces every step in def.steps onto workflow-Engine's
// Redis Stream, in slice order (see the DependsOn note on the type doc — no
// dependency-graph sequencing happens here). NOTE: as of this writing,
// workflow-Engine's worker pool only has the faultinject demo's order-saga
// steps registered, so a StepMessage naming any other tool will be produced
// successfully but fail on the worker side with ErrStepNotRegistered until
// a real StepExecutor is registered for it upstream.
func (s *WorkflowEngineService) ExecuteWorkflow(ctx context.Context, id string) error {
	s.mu.RLock()
	def, ok := s.defs[id]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}

	for _, step := range def.steps {
		msg := wfengine.StepMessage{
			WorkflowID: id,
			StepName:   step.Tool,
			Input:      step.Input,
		}
		if err := s.queue.ProduceStep(ctx, msg); err != nil {
			return fmt.Errorf("produce step %s: %w", step.Tool, err)
		}
	}
	return nil
}

func (s *WorkflowEngineService) GetWorkflow(id string) (*api.Workflow, error) {
	ctx := context.Background()
	wfID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow id: %w", err)
	}

	wf, err := s.store.GetWorkflow(ctx, wfID)
	if err != nil {
		return nil, err
	}

	events, err := s.store.ReplayEvents(ctx, wfID)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	def, hasDef := s.defs[id]
	s.mu.RUnlock()

	var steps []*api.WorkflowStep
	query := ""
	if hasDef {
		steps = applyEventsToSteps(def.steps, events)
		query = def.query
	}

	return &api.Workflow{
		ID:        id,
		Name:      wf.Name,
		Query:     query,
		Steps:     steps,
		Status:    wf.Status,
		CreatedAt: wf.CreatedAt,
		UpdatedAt: wf.UpdatedAt,
	}, nil
}

func (s *WorkflowEngineService) ListWorkflows() []*api.Workflow {
	ctx := context.Background()
	wfs, err := s.store.ListWorkflows(ctx, 50)
	if err != nil {
		s.logger.Error("list workflows", zap.Error(err))
		return nil
	}

	out := make([]*api.Workflow, 0, len(wfs))
	for _, wf := range wfs {
		full, err := s.GetWorkflow(wf.ID.String())
		if err != nil {
			s.logger.Warn("skipping workflow in list", zap.String("id", wf.ID.String()), zap.Error(err))
			continue
		}
		out = append(out, full)
	}
	return out
}

func (s *WorkflowEngineService) GetEvents(id string) ([]*api.WorkflowEvent, error) {
	wfID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow id: %w", err)
	}
	events, err := s.store.ReplayEvents(context.Background(), wfID)
	if err != nil {
		return nil, err
	}

	out := make([]*api.WorkflowEvent, len(events))
	for i, e := range events {
		var payload map[string]interface{}
		_ = json.Unmarshal(e.Payload, &payload)
		out[i] = &api.WorkflowEvent{
			ID:         fmt.Sprintf("%d", e.ID),
			WorkflowID: e.WorkflowID.String(),
			StepID:     e.StepName,
			Type:       e.Type,
			Payload:    payload,
			DedupKey:   e.DedupKey,
			Timestamp:  e.CreatedAt,
		}
	}
	return out, nil
}

func (s *WorkflowEngineService) GetStep(workflowID, stepID string) (*api.WorkflowStep, error) {
	s.mu.RLock()
	def, ok := s.defs[workflowID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	for _, step := range def.steps {
		if step.ID == stepID {
			wfID, err := uuid.Parse(workflowID)
			if err != nil {
				return step, nil
			}
			events, err := s.store.ReplayEvents(context.Background(), wfID)
			if err != nil {
				return step, nil
			}
			return applyEventsToSteps([]*api.WorkflowStep{step}, events)[0], nil
		}
	}
	return nil, fmt.Errorf("step not found: %s", stepID)
}

// applyEventsToSteps overlays the latest matching event onto each step's
// Status/Output/Error, matching events to steps by step_name == step.Tool
// (the name workflow-Engine's worker registry looks steps up by).
func applyEventsToSteps(steps []*api.WorkflowStep, events []wfengine.Event) []*api.WorkflowStep {
	latest := make(map[string]wfengine.Event)
	for _, e := range events {
		latest[e.StepName] = e // events are in commit order; last write wins
	}

	out := make([]*api.WorkflowStep, len(steps))
	for i, step := range steps {
		s := *step // shallow copy so callers' cached def isn't mutated by concurrent readers
		if e, ok := latest[step.Tool]; ok {
			var payload map[string]interface{}
			_ = json.Unmarshal(e.Payload, &payload)
			switch e.Type {
			case "step_completed":
				s.Status = "completed"
				s.Output = payload
			case "step_failed":
				s.Status = "failed"
				if payload != nil {
					if errMsg, ok := payload["error"].(string); ok {
						s.Error = errMsg
					}
				}
			case "step_started":
				s.Status = "running"
			}
		}
		out[i] = &s
	}
	return out
}

// BuildDemoWorkflow returns a naturally sequential pipeline (no DependsOn
// branching — see the package doc for why) for the target-validation-
// researcher persona's killer query: retrieve literature grounding, then
// synthesize. Step Tool names must match a StepExecutor registered on
// workflow-Engine's worker to actually execute; see the package doc's note
// on ErrStepNotRegistered for the current state of that registry.
func BuildDemoWorkflow(query string) (*api.Workflow, []*api.WorkflowStep) {
	steps := []*api.WorkflowStep{
		{ID: "retrieve", Name: "Literature Retrieval", Category: "retriever", Tool: "biolab-retrieve", Input: map[string]interface{}{"query": query}, MaxRetries: 3},
		{ID: "synthesize", Name: "Confidence-Scored Synthesis", Category: "analyzer", Tool: "synthesize", Input: map[string]interface{}{"query": query}, MaxRetries: 1},
	}
	return nil, steps
}
