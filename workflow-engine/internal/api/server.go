package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/srikarjy/research-orchestrator/workflow-engine/internal/engine"
	"github.com/srikarjy/research-orchestrator/workflow-engine/pkg/eventlog"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	router  *mux.Router
	eng     *engine.Engine
	logger  *zap.Logger
	store   eventlog.Store
	wsClients map[string]map[*websocket.Conn]bool
}

func NewServer(eng *engine.Engine, store eventlog.Store, logger *zap.Logger) *Server {
	s := &Server{
		router:    mux.NewRouter(),
		eng:       eng,
		logger:    logger,
		store:     store,
		wsClients: make(map[string]map[*websocket.Conn]bool),
	}
	s.routes()
	return s
}

func (s *Server) Router() *mux.Router {
	return s.router
}

func (s *Server) routes() {
	s.router.HandleFunc("/health", s.health).Methods("GET")
	s.router.HandleFunc("/api/v1/workflows", s.createWorkflow).Methods("POST")
	s.router.HandleFunc("/api/v1/workflows", s.listWorkflows).Methods("GET")
	s.router.HandleFunc("/api/v1/workflows/{id}", s.getWorkflow).Methods("GET")
	s.router.HandleFunc("/api/v1/workflows/{id}/execute", s.executeWorkflow).Methods("POST")
	s.router.HandleFunc("/api/v1/workflows/{id}/events", s.getWorkflowEvents).Methods("GET")
	s.router.HandleFunc("/api/v1/workflows/{id}/steps/{stepId}", s.getStep).Methods("GET")
	s.router.HandleFunc("/api/v1/workflows/{id}/ws", s.handleWorkflowWS).Methods("GET")
	
	// Executor endpoints (calendar, notifications, running tasks)
	s.router.HandleFunc("/api/v1/executor/calendar", s.getCalendarEvents).Methods("GET")
	s.router.HandleFunc("/api/v1/executor/notifications", s.getNotifications).Methods("GET")
	s.router.HandleFunc("/api/v1/executor/tasks", s.getRunningTasks).Methods("GET")
	s.router.HandleFunc("/api/v1/executor/notifications/{id}/read", s.markNotificationRead).Methods("POST")
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

type CreateWorkflowRequest struct {
	Name  string                 `json:"name"`
	Query string                 `json:"query"`
	Steps []*engine.Step         `json:"steps,omitempty"`
}

func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		req.Name = "Research Investigation"
	}
	if req.Query == "" {
		s.error(w, http.StatusBadRequest, "query is required")
		return
	}

	steps := req.Steps
	if len(steps) == 0 {
		_, steps = engine.BuildDemoWorkflow(req.Query)
	}

	wf, err := s.eng.CreateWorkflow(r.Context(), req.Name, req.Query, steps)
	if err != nil {
		s.logger.Error("create workflow failed", zap.Error(err))
		s.error(w, http.StatusInternalServerError, "failed to create workflow")
		return
	}

	s.respond(w, http.StatusCreated, wf)
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	// In production, this would query a workflow index
	workflows := make([]*engine.Workflow, 0, len(s.eng.GetAllWorkflows()))
	for _, wf := range s.eng.GetAllWorkflows() {
		workflows = append(workflows, wf)
	}
	s.respond(w, http.StatusOK, workflows)
}

func (s *Server) getWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	wf, err := s.eng.GetWorkflow(id)
	if err != nil {
		s.error(w, http.StatusNotFound, "workflow not found")
		return
	}
	s.respond(w, http.StatusOK, wf)
}

func (s *Server) executeWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	// Execute asynchronously
	go func() {
		if err := s.eng.ExecuteWorkflow(r.Context(), id); err != nil {
			s.logger.Error("workflow execution failed", zap.String("workflow_id", id), zap.Error(err))
		}
	}()
	
	s.respond(w, http.StatusAccepted, map[string]string{"status": "execution started"})
}

func (s *Server) getWorkflowEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	events, err := s.eng.GetWorkflowEvents(id)
	if err != nil {
		s.error(w, http.StatusInternalServerError, "failed to fetch events")
		return
	}
	s.respond(w, http.StatusOK, events)
}

func (s *Server) getStep(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	stepID := vars["stepId"]
	
	wf, err := s.eng.GetWorkflow(id)
	if err != nil {
		s.error(w, http.StatusNotFound, "workflow not found")
		return
	}
	
	for _, step := range wf.Steps {
		if step.ID == stepID {
			s.respond(w, http.StatusOK, step)
			return
		}
	}
	s.error(w, http.StatusNotFound, "step not found")
}

// Executor endpoints - mock data for now, replace with real executor service calls
func (s *Server) getCalendarEvents(w http.ResponseWriter, r *http.Request) {
	events := []map[string]interface{}{
		{
			"id": "cal-001", "title": "Weekly Lab Meeting",
			"start": "2026-08-01T10:00:00-07:00", "end": "2026-08-01T11:00:00-07:00",
			"source": "google", "calendarId": "lab-team@company.com",
			"meetingUrl": "https://meet.google.com/abc-def-ghi",
			"attendees": []string{"pi@company.com", "postdoc1@company.com"},
		},
		{
			"id": "cal-002", "title": "BRAF V600E Docking Run",
			"start": "2026-08-01T14:00:00-07:00", "end": "2026-08-01T16:00:00-07:00",
			"source": "workflow", "calendarId": "workflow-engine",
		},
		{
			"id": "cal-003", "title": "Clinical Data Review",
			"start": "2026-08-02T09:30:00-07:00", "end": "2026-08-02T10:15:00-07:00",
			"source": "mac", "calendarId": "personal",
			"attendees": []string{"chen@hospital.org"},
		},
	}
	s.respond(w, http.StatusOK, events)
}

func (s *Server) getNotifications(w http.ResponseWriter, r *http.Request) {
	notifications := []map[string]interface{}{
		{
			"id": "notif-001", "type": "task_running",
			"title": "Protein Stability Analysis",
			"message": "Predicting ΔΔG for BRAF V600E mutation — step 3/5 (MD relaxation)",
			"timestamp": "2026-08-01T08:23:12-07:00", "read": false,
			"relatedEvidenceId": "EV-0042", "relatedToolCall": "ProteinStabilityPredictor",
		},
		{
			"id": "notif-002", "type": "review_required",
			"title": "Human Review Required",
			"message": "Critic flagged contradiction: PDB 4RZW shows DFG-in but 6MUK shows DFG-out for V600E. Confidence gated at 0.68.",
			"timestamp": "2026-08-01T08:15:44-07:00", "read": false,
			"relatedEvidenceId": "EV-0042", "actionUrl": "/evidence/EV-0042/contradictions",
		},
		{
			"id": "notif-003", "type": "task_completed",
			"title": "PubMed Retrieval Complete",
			"message": "Found 12 new papers on BRAF inhibitor resistance mechanisms. 3 high-confidence hits.",
			"timestamp": "2026-08-01T07:52:10-07:00", "read": true,
			"relatedEvidenceId": "EV-0042", "relatedToolCall": "PubMed",
		},
	}
	s.respond(w, http.StatusOK, notifications)
}

func (s *Server) getRunningTasks(w http.ResponseWriter, r *http.Request) {
	tasks := []map[string]interface{}{
		{
			"id": "task-001", "name": "Protein Stability Prediction (BRAF V600E)",
			"startedAt": "2026-08-01T08:20:00-07:00", "status": "running",
			"progress": 0.6, "currentStep": "MD relaxation (3/5)", "evidenceId": "EV-0042",
		},
		{
			"id": "task-002", "name": "Docking: Vemurafenib → BRAF V600E",
			"startedAt": "2026-08-01T08:22:00-07:00", "status": "pending",
			"progress": 0.0, "currentStep": "Queued (awaiting GPU)", "evidenceId": "EV-0042",
		},
		{
			"id": "task-003", "name": "Literature Contradiction Review",
			"startedAt": "2026-08-01T08:15:00-07:00", "status": "awaiting_review",
			"progress": 1.0, "currentStep": "Awaiting human decision", "evidenceId": "EV-0042",
		},
	}
	s.respond(w, http.StatusOK, tasks)
}

func (s *Server) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	// In production, update notification in database
	s.respond(w, http.StatusOK, map[string]string{"id": id, "read": "true"})
}

func (s *Server) respond(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) error(w http.ResponseWriter, status int, msg string) {
	s.respond(w, status, map[string]string{"error": msg})
}

// handleWorkflowWS handles WebSocket connections for real-time workflow events
func (s *Server) handleWorkflowWS(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]
	
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()
	
	// Register client
	if s.wsClients[workflowID] == nil {
		s.wsClients[workflowID] = make(map[*websocket.Conn]bool)
	}
	s.wsClients[workflowID][conn] = true
	defer func() {
		delete(s.wsClients[workflowID], conn)
		if len(s.wsClients[workflowID]) == 0 {
			delete(s.wsClients, workflowID)
		}
	}()
	
	// Send initial workflow state
	wf, err := s.eng.GetWorkflow(workflowID)
	if err == nil {
		conn.WriteJSON(map[string]interface{}{
			"type": "workflow_state",
			"data": wf,
		})
	}
	
	// Send existing events
	events, _ := s.eng.GetWorkflowEvents(workflowID)
	for _, evt := range events {
		conn.WriteJSON(map[string]interface{}{
			"type": "event",
			"data": evt,
		})
	}
	
	// Keep connection alive and listen for close
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// BroadcastEvent sends an event to all WebSocket clients for a workflow
func (s *Server) BroadcastEvent(workflowID string, event *eventlog.Event) {
	clients := s.wsClients[workflowID]
	for conn := range clients {
		err := conn.WriteJSON(map[string]interface{}{
			"type": "event",
			"data": event,
		})
		if err != nil {
			s.logger.Error("WebSocket write failed", zap.Error(err))
			conn.Close()
			delete(s.wsClients[workflowID], conn)
		}
	}
}