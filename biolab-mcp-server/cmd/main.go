package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/google/uuid"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/agents"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/mcp"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/sandbox"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/shared"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/tools/analyzers"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/tools/retrievers"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/tools/visualizers"
	"go.uber.org/zap"
)

func main() {
	godotenv.Load()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Create tool registry
	registry := mcp.NewToolRegistry()

	// Register retrievers
	registry.Register(retrievers.NewPubMedRetriever())
	registry.Register(retrievers.NewUniProtRetriever())
	registry.Register(retrievers.NewChEMBLRetriever())
	registry.Register(retrievers.NewPDBRetriever())
	registry.Register(retrievers.NewKEGGRetriever())
	registry.Register(retrievers.NewBindingDBRetriever())

	// Register analyzers
	registry.Register(analyzers.NewProteinStabilityPredictor())
	registry.Register(analyzers.NewDockingAnalyzer())
	registry.Register(analyzers.NewEvidenceMerge())
	registry.Register(analyzers.NewCriticAgent())

	// Register visualizers
	registry.Register(visualizers.NewStructureViewer())
	registry.Register(visualizers.NewMoleculeViewer())

	// Create message bus
	msgBus := agents.NewInMemoryMessageBus()

	// Create sandbox
	sandboxConfig := sandbox.SandboxConfig{
		BasePath:       getEnv("SANDBOX_PATH", "/tmp/research-sandbox"),
		MaxConcurrent:  10,
		DefaultTimeout: 30 * time.Minute,
	}
	sb := sandbox.NewSandbox(sandboxConfig, logger)

	// Create agent factory
	emailConfig := agents.EmailConfig{
		SMTPHost:    getEnv("SMTP_HOST", ""),
		SMTPPort:    587,
		Username:    getEnv("SMTP_USERNAME", ""),
		Password:    getEnv("SMTP_PASSWORD", ""),
		FromAddress: getEnv("SMTP_FROM", "noreply@research-orchestrator.local"),
		FromName:    "Research Orchestrator",
		UseTLS:      true,
	}

	factory := agents.NewAgentFactory(msgBus, registry, sb, logger, emailConfig)

	// Create and register all agents
	agentList, err := factory.CreateAllAgents()
	if err != nil {
		logger.Fatal("Failed to create agents", zap.Error(err))
	}

	orchestrator := factory.CreateOrchestrator()
	for _, agent := range agentList {
		if err := orchestrator.RegisterAgent(agent); err != nil {
			logger.Error("Failed to register agent", zap.Error(err), zap.String("agent", agent.Name()))
		}
	}

	// Create HTTP server
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Agent endpoints
	router.HandleFunc("/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		agentList := orchestrator.ListAgents()
		result := make([]map[string]interface{}, len(agentList))
		for i, a := range agentList {
			result[i] = map[string]interface{}{
				"id":           a.ID(),
				"name":         a.Name(),
				"description":  a.Description(),
				"capabilities": a.Capabilities(),
				"status":       a.Status(),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}).Methods("GET")

	router.HandleFunc("/api/v1/agents/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := agents.AgentID(vars["id"])
		agent, ok := orchestrator.GetAgent(id)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "agent not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     agent.ID(),
			"name":   agent.Name(),
			"status": agent.Status(),
		})
	}).Methods("GET")

	// Workflow endpoints
	router.HandleFunc("/api/v1/workflows", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Tasks       []agents.Task          `json:"tasks"`
				Metadata    map[string]interface{} `json:"metadata"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
				return
			}

			workflow := orchestrator.CreateWorkflow(req.Name, req.Description, req.Tasks)
			workflow.Metadata = req.Metadata

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(workflow)
			return
		}

		workflows := orchestrator.ListWorkflows()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(workflows)
	}).Methods("GET", "POST")

	router.HandleFunc("/api/v1/workflows/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		if r.Method == "DELETE" {
			if err := orchestrator.CancelWorkflow(id); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		workflow, ok := orchestrator.GetWorkflow(id)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "workflow not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(workflow)
	}).Methods("GET", "DELETE")

	router.HandleFunc("/api/v1/workflows/{id}/execute", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		go func() {
			ctx := context.Background()
			if err := orchestrator.ExecuteWorkflow(ctx, id); err != nil {
				logger.Error("Workflow execution failed", zap.String("id", id), zap.Error(err))
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "started", "workflow_id": id})
	}).Methods("POST")

	// Sandbox endpoints
	router.HandleFunc("/api/v1/sandbox/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req struct {
				ExperimentID string                 `json:"experiment_id"`
				Metadata     map[string]interface{} `json:"metadata"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
				return
			}

			session, err := sb.CreateSession(r.Context(), req.ExperimentID, req.Metadata)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(session)
			return
		}

		sessions := sb.ListSessions()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	}).Methods("GET", "POST")

	router.HandleFunc("/api/v1/sandbox/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		if r.Method == "DELETE" {
			if err := sb.CleanupSession(id); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		session, ok := sb.GetSession(id)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "session not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(session)
	}).Methods("GET", "DELETE")

	router.HandleFunc("/api/v1/sandbox/sessions/{id}/execute", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var req shared.ExperimentSpec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
			return
		}

		session, err := sb.RunExperiment(r.Context(), id, req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(session)
	}).Methods("POST")

	// Tool endpoints (existing)
	router.HandleFunc("/api/v1/tools", func(w http.ResponseWriter, r *http.Request) {
		tools := registry.List()
		result := make([]map[string]interface{}, len(tools))
		for i, t := range tools {
			result[i] = map[string]interface{}{
				"name":         t.Name(),
				"category":     t.Category(),
				"description":  t.Description(),
				"input_schema": t.InputSchema(),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}).Methods("GET")

	router.HandleFunc("/api/v1/tools/{category}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		category := vars["category"]
		tools := registry.ListByCategory(category)
		result := make([]map[string]interface{}, len(tools))
		for i, t := range tools {
			result[i] = map[string]interface{}{
				"name":         t.Name(),
				"category":     t.Category(),
				"description":  t.Description(),
				"input_schema": t.InputSchema(),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}).Methods("GET")

	router.HandleFunc("/api/v1/tools/{category}/{name}/execute", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		category := vars["category"]
		name := vars["name"]

		tool, ok := registry.Get(category, name)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "tool not found"})
			return
		}

		var input map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		result, err := tool.Execute(ctx, input)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}).Methods("POST")

	router.HandleFunc("/api/v1/tools/{category}/{name}/schema", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		category := vars["category"]
		name := vars["name"]

		tool, ok := registry.Get(category, name)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "tool not found"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tool.InputSchema())
	}).Methods("GET")

	// Notification endpoints
	router.HandleFunc("/api/v1/notifications/send", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			NotificationType string                 `json:"notification_type"`
			Recipients       []string               `json:"recipients"`
			Channels         []string               `json:"channels"`
			Data             map[string]interface{} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
			return
		}

		notifierAgent, ok := orchestrator.GetAgent(agents.AgentNotifier)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "notifier agent not found"})
			return
		}

		task := agents.Task{
			ID:   uuid.New().String(),
			Type: "notify",
			Input: map[string]interface{}{
				"notification_type": req.NotificationType,
				"recipients":        req.Recipients,
				"channels":          req.Channels,
				"data":              req.Data,
			},
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := notifierAgent.Execute(ctx, task)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result.Output)
	}).Methods("POST")

	addr := getEnv("PORT", "8081")
	server := &http.Server{
		Addr:         ":" + addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("Shutting down Biolab MCP Server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	logger.Info("Biolab MCP Server starting", 
		zap.String("addr", addr), 
		zap.Int("tools", len(registry.List())),
		zap.Int("agents", len(agentList)))

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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