package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/srikarjy/research-orchestrator/workflow-engine/internal/api"
	"github.com/srikarjy/research-orchestrator/workflow-engine/internal/engine"
	"github.com/srikarjy/research-orchestrator/workflow-engine/pkg/eventlog"
	"go.uber.org/zap"
)

func main() {
	// Load .env if exists
	godotenv.Load()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize event log store (in-memory for demo)
	logger.Info("Using in-memory event store (set DATABASE_URL for PostgreSQL)")
	memStore := eventlog.NewMemoryStore()
	store := memStore
	// Seed demo data
	demoEvents := eventlog.GenerateTestEvents("demo-workflow")
	memStore.SeedWorkflow("demo-workflow", demoEvents)

	// Create engine
	eng := engine.NewEngine(store)

	// Register mock executors for demo
	eng.RegisterExecutor(&MockExecutor{ToolName: "PubMed", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "UniProt", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "ChEMBL", Category: "retriever"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "ProteinStabilityPredictor", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "Docking", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "EvidenceMerge", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "Critic", Category: "analyzer"})
	eng.RegisterExecutor(&MockExecutor{ToolName: "StructureViewer", Category: "visualizer"})

	// Create API server
	server := api.NewServer(eng, store, logger)

	// Set event callback for WebSocket broadcasting
	eng.SetEventCallback(server.BroadcastEvent)

	// HTTP server
	addr := getEnv("PORT", "8080")
	httpServer := &http.Server{
		Addr:         ":" + addr,
		Handler:      server.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
		store.Close()
	}()

	logger.Info("Workflow Engine starting", zap.String("addr", addr))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed", zap.Error(err))
	}
	logger.Info("Server stopped")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// MockExecutor simulates tool execution for demo
type MockExecutor struct {
	ToolName string
	Category string
}

func (m *MockExecutor) Execute(ctx context.Context, step *engine.Step) (map[string]interface{}, error) {
	// Simulate work
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}
	return map[string]interface{}{
		"tool":      m.ToolName,
		"status":    "completed",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"simulated": true,
	}, nil
}

func (m *MockExecutor) GetToolName() string { return m.ToolName }
func (m *MockExecutor) GetCategory() string { return m.Category }