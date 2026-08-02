package eventlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventTypeWorkflowStarted EventType = "workflow_started"
	EventTypeStepStarted     EventType = "step_started"
	EventTypeStepCompleted   EventType = "step_completed"
	EventTypeStepFailed      EventType = "step_failed"
	EventTypeStepRetried     EventType = "step_retried"
	EventTypeWorkflowCompleted EventType = "workflow_completed"
	EventTypeWorkflowFailed  EventType = "workflow_failed"
	EventTypeHumanReviewRequested EventType = "human_review_requested"
	EventTypeHumanReviewResolved EventType = "human_review_resolved"
	EventTypeNotificationSent EventType = "notification_sent"
	EventTypeCalendarEventCreated EventType = "calendar_event_created"
)

type Event struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	StepID      string                 `json:"step_id,omitempty"`
	Type        EventType              `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	Payload     map[string]interface{} `json:"payload"`
	DedupKey    string                 `json:"dedup_key"`
	IdempotencyKey string              `json:"idempotency_key,omitempty"`
	PrevEventID string                 `json:"prev_event_id,omitempty"`
}

func NewEvent(workflowID, stepID string, typ EventType, payload map[string]interface{}, idempotencyKey string) *Event {
	dedupKey := GenerateDedupKey(workflowID, stepID, typ, payload)
	return &Event{
		ID:              uuid.New().String(),
		WorkflowID:      workflowID,
		StepID:          stepID,
		Type:            typ,
		Timestamp:       time.Now().UTC(),
		Payload:         payload,
		DedupKey:        dedupKey,
		IdempotencyKey:  idempotencyKey,
	}
}

func GenerateDedupKey(workflowID, stepID string, typ EventType, payload map[string]interface{}) string {
	data := map[string]interface{}{
		"workflow_id": workflowID,
		"step_id":     stepID,
		"type":        typ,
		"payload":     payload,
	}
	bytes, _ := json.Marshal(data)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

func (e *Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	return json.Marshal(&struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias:     (*Alias)(e),
		Timestamp: e.Timestamp.Format(time.RFC3339Nano),
	})
}