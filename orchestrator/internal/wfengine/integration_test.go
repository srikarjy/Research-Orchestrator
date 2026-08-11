package wfengine

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// TestIntegration_RoundTripAgainstRealWorkflowEngine is the guardrail flagged
// when this package was written: the SQL and StepMessage shape in client.go
// are hand-mirrored from github.com/srikarjy/workflow-Engine's real schema
// and wire format, with nothing forcing them to stay in sync. This test
// drives the actual docker-compose.yml services (real migrate + real
// worker, built from the workflow-engine submodule, not a mock) through
// this package's client code exactly as orchestrator's production code
// does, and fails the moment either side's schema or wire format drifts.
//
// Opt-in via WFENGINE_INTEGRATION=1 (needs Docker) -- not run by default so
// `go test ./...` doesn't require Docker everywhere this repo is built.
func TestIntegration_RoundTripAgainstRealWorkflowEngine(t *testing.T) {
	if os.Getenv("WFENGINE_INTEGRATION") != "1" {
		t.Skip("set WFENGINE_INTEGRATION=1 to run (requires Docker; brings up the real workflow-engine-worker via docker-compose.yml)")
	}

	repoRoot := "../../.." // orchestrator/internal/wfengine -> repo root
	compose := func(args ...string) error {
		cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
		cmd.Dir = repoRoot
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if err := compose("up", "-d", "--build", "postgres", "redis", "workflow-engine-migrate", "workflow-engine-worker"); err != nil {
		t.Fatalf("docker compose up: %v", err)
	}
	t.Cleanup(func() {
		if err := compose("down"); err != nil {
			t.Logf("docker compose down: %v (not failing the test over cleanup)", err)
		}
	})

	const dsn = "postgres://workflow:workflow@localhost:5434/workflow?sslmode=disable"
	const stream = "workflow-steps"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	store := waitForStore(t, ctx, dsn)
	defer store.Close()

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer rdb.Close()
	queue := NewQueue(rdb, stream)

	wfID := uuid.New()
	if err := store.CreateWorkflow(ctx, wfID, "integration-guardrail", nil); err != nil {
		t.Fatalf("CreateWorkflow: %v (schema drift? see migrations/0001_init.up.sql vs client.go's INSERT)", err)
	}

	// reserve_inventory is one of the real order-saga demo steps the worker
	// has registered -- see workflow-engine/internal/steps/order_saga.go.
	err := queue.ProduceStep(ctx, StepMessage{
		WorkflowID: wfID.String(),
		StepName:   "reserve_inventory",
		Input:      map[string]any{"sku": "integration-test"},
	})
	if err != nil {
		t.Fatalf("ProduceStep: %v (wire format drift? see StepMessage vs internal/queue.StepMessage)", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for {
		events, err := store.ReplayEvents(ctx, wfID)
		if err != nil {
			t.Fatalf("ReplayEvents: %v", err)
		}
		for _, e := range events {
			if e.StepName == "reserve_inventory" && e.Type == "step_completed" {
				return // round trip proven: real worker processed our real StepMessage
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for step_completed; got events: %+v (worker not consuming? see StepMessage/stream/group vs internal/queue.go and cmd/worker defaults)", events)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForStore(t *testing.T, ctx context.Context, dsn string) *Store {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		store, err := NewStore(ctx, dsn)
		if err == nil {
			return store
		}
		lastErr = err
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("postgres never became reachable: %v", lastErr)
	return nil
}
