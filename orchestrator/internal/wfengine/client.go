// Package wfengine talks to the standalone github.com/srikarjy/workflow-Engine
// service over its real durable boundaries — its Postgres event log and the
// Redis Stream its worker pool consumes — instead of importing its Go
// packages. A direct import isn't possible: workflow-Engine's execution
// logic (internal/engine, internal/store, internal/queue) lives under
// internal/, which Go forbids importing across module boundaries, and
// promoting it to pkg/ was deliberately rejected because the crash-recovery
// guarantee that repo proves via faultinject only covers its queue-consumer
// path (ProcessStep), not an in-process call — see that repo's engine.go.
// Going through the queue is what lets orchestrator inherit that proof
// instead of introducing a second, unproven execution path.
//
// KNOWN RISK: the SQL and the StepMessage shape below are hand-mirrored
// from workflow-Engine's internal/store/store.go, migrations/0001_init.up.sql,
// and internal/queue/queue.go. Nothing forces this file to stay in sync if
// that schema or wire shape changes upstream. Tracked follow-up: either an
// integration test that round-trips a workflow/step through both sides, or
// promoting just the StepMessage type (data shape only, no behavior) into a
// pkg/contract package workflow-Engine exports.
package wfengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Workflow mirrors workflow-Engine's internal/store.Workflow.
type Workflow struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Status    string          `json:"status"`
	Input     json.RawMessage `json:"input"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Event mirrors workflow-Engine's internal/store.Event.
type Event struct {
	ID         int64           `json:"id"`
	WorkflowID uuid.UUID       `json:"workflow_id"`
	StepName   string          `json:"step_name"`
	Type       string          `json:"event_type"`
	DedupKey   string          `json:"dedup_key"`
	Payload    json.RawMessage `json:"payload"`
	CreatedAt  time.Time       `json:"created_at"`
}

// StepMessage mirrors workflow-Engine's internal/queue.StepMessage exactly
// (field names and JSON tags matter: this is decoded by queue.ParseStepMessage
// on the other side). DedupKey may be left empty — ProcessStep computes it
// from (WorkflowID, StepName, Input) when absent.
type StepMessage struct {
	WorkflowID string         `json:"workflow_id"`
	StepName   string         `json:"step_name"`
	Input      map[string]any `json:"input"`
	DedupKey   string         `json:"dedup_key"`
	RetryCount int            `json:"retry_count"`
}

// ErrNotFound mirrors workflow-Engine's internal/store.ErrNotFound.
var ErrNotFound = errors.New("wfengine: workflow not found")

// Store reads and writes workflow-Engine's Postgres event log directly.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore opens a connection pool to workflow-Engine's own Postgres
// database (not orchestrator's — see the WorkflowEngine.DSN default in
// kernel.Config, which points at workflow-Engine's docker-compose Postgres).
func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("wfengine: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("wfengine: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

// CreateWorkflow inserts a workflow row, mirroring internal/store.Store.CreateWorkflow.
// Idempotent on id via ON CONFLICT DO NOTHING, matching upstream.
func (s *Store) CreateWorkflow(ctx context.Context, id uuid.UUID, name string, input json.RawMessage) error {
	if input == nil {
		input = json.RawMessage("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO workflows (id, name, status, input)
		VALUES ($1, $2, 'running', $3)
		ON CONFLICT (id) DO NOTHING
	`, id, name, input)
	return err
}

// GetWorkflow mirrors internal/store.Store.GetWorkflow.
func (s *Store) GetWorkflow(ctx context.Context, id uuid.UUID) (Workflow, error) {
	var w Workflow
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, status, input, created_at, updated_at
		FROM workflows WHERE id = $1
	`, id).Scan(&w.ID, &w.Name, &w.Status, &w.Input, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workflow{}, ErrNotFound
	}
	return w, err
}

// ListWorkflows mirrors cmd/cli's listCmd query.
func (s *Store) ListWorkflows(ctx context.Context, limit int) ([]Workflow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, status, input, created_at, updated_at
		FROM workflows ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Workflow
	for rows.Next() {
		var w Workflow
		if err := rows.Scan(&w.ID, &w.Name, &w.Status, &w.Input, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ReplayEvents mirrors internal/store.Store.ReplayEvents — every event for
// a workflow in commit order, the same log the worker pool's crash recovery
// replays.
func (s *Store) ReplayEvents(ctx context.Context, workflowID uuid.UUID) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workflow_id, step_name, event_type, dedup_key, payload, created_at
		FROM events WHERE workflow_id = $1 ORDER BY id ASC
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.WorkflowID, &e.StepName, &e.Type, &e.DedupKey, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Queue produces StepMessages onto the Redis Stream workflow-Engine's
// worker pool consumes. Note: as of this writing, that worker pool's
// StepRegistry only has the faultinject demo's order-saga steps registered
// (see steps.OrderSagaSteps() upstream) — a StepMessage with any other
// StepName will be produced successfully but fail on the worker side with
// ErrStepNotRegistered until a real StepExecutor is registered for it.
type Queue struct {
	rdb    *redis.Client
	stream string
}

// NewQueue wraps an existing Redis client (orchestrator's own — shared with
// workflow-Engine's worker once the two docker-compose files point at one
// Redis instance) for producing onto the given stream.
func NewQueue(rdb *redis.Client, stream string) *Queue {
	return &Queue{rdb: rdb, stream: stream}
}

// ProduceStep mirrors internal/queue.Client.ProduceStep's wire format
// exactly: the StepMessage JSON goes under a "data" field, which is what
// queue.ParseStepMessage on the consumer side expects.
func (q *Queue) ProduceStep(ctx context.Context, msg StepMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{"data": string(data)},
	}).Err()
}
