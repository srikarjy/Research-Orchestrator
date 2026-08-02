package eventlog

import (
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.RWMutex
	events  map[string]*Event
	byWF    map[string][]*Event
	byDedup map[string]*Event
	byIdemp map[string]*Event
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		events:  make(map[string]*Event),
		byWF:    make(map[string][]*Event),
		byDedup: make(map[string]*Event),
		byIdemp: make(map[string]*Event),
	}
}

func (s *MemoryStore) Append(event *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byDedup[event.DedupKey]; exists {
		return ErrDuplicateDedupKey
	}
	if event.IdempotencyKey != "" {
		if _, exists := s.byIdemp[event.IdempotencyKey]; exists {
			return ErrDuplicateDedupKey
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

func (s *MemoryStore) GetByWorkflowID(workflowID string) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.byWF[workflowID]
	result := make([]*Event, len(events))
	copy(result, events)
	return result, nil
}

func (s *MemoryStore) GetByDedupKey(dedupKey string) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.byDedup[dedupKey]; ok {
		return e, nil
	}
	return nil, ErrEventNotFound
}

func (s *MemoryStore) GetByIdempotencyKey(key string) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.byIdemp[key]; ok {
		return e, nil
	}
	return nil, ErrEventNotFound
}

func (s *MemoryStore) GetLatestEvent(workflowID string) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.byWF[workflowID]
	if len(events) == 0 {
		return nil, ErrEventNotFound
	}
	return events[len(events)-1], nil
}

func (s *MemoryStore) Close() error {
	return nil
}

func (s *MemoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = make(map[string]*Event)
	s.byWF = make(map[string][]*Event)
	s.byDedup = make(map[string]*Event)
	s.byIdemp = make(map[string]*Event)
}

func (s *MemoryStore) SeedWorkflow(workflowID string, events []*Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range events {
		s.events[e.ID] = e
		s.byWF[workflowID] = append(s.byWF[workflowID], e)
		s.byDedup[e.DedupKey] = e
		if e.IdempotencyKey != "" {
			s.byIdemp[e.IdempotencyKey] = e
		}
	}
}

func GenerateTestEvents(workflowID string) []*Event {
	now := time.Now().UTC()
	return []*Event{
		{
			ID:         "evt-1",
			WorkflowID: workflowID,
			Type:       EventTypeWorkflowStarted,
			Timestamp:  now,
			Payload:    map[string]interface{}{"query": "BRAF V600E binding affinity"},
			DedupKey:   "dedup-wf-start",
		},
		{
			ID:         "evt-2",
			WorkflowID: workflowID,
			StepID:     "pubmed",
			Type:       EventTypeStepStarted,
			Timestamp:  now.Add(1 * time.Second),
			Payload:    map[string]interface{}{"tool": "PubMed"},
			DedupKey:   "dedup-pubmed-start",
		},
		{
			ID:         "evt-3",
			WorkflowID: workflowID,
			StepID:     "pubmed",
			Type:       EventTypeStepCompleted,
			Timestamp:  now.Add(2 * time.Second),
			Payload:    map[string]interface{}{"tool": "PubMed", "results": 12, "latency_ms": 420},
			DedupKey:   "dedup-pubmed-complete",
		},
	}
}