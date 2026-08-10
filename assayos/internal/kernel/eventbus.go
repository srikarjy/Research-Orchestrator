package kernel

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type EventBus struct {
	logger   *zap.Logger
	mu       sync.RWMutex
	subs     map[string][]EventHandler
	wildcard []EventHandler
	running  bool
	metrics  *EventMetrics
}

type EventHandler func(ctx context.Context, event Event) error

type Event struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`       // plane: workflow, biolab, aletheia
	WorkflowID  string                 `json:"workflow_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	Payload     map[string]interface{} `json:"payload"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

type EventMetrics struct {
	mu           sync.Mutex
	Published    int64
	Delivered    int64
	Failed       int64
	ByType       map[string]int64
	BySource     map[string]int64
	LatencySumMs int64
	LatencyCount int64
}

func NewEventBus(logger *zap.Logger) *EventBus {
	return &EventBus{
		logger:   logger.Named("eventbus"),
		subs:     make(map[string][]EventHandler),
		wildcard: make([]EventHandler, 0),
		metrics:  &EventMetrics{ByType: make(map[string]int64), BySource: make(map[string]int64)},
		running:  false,
	}
}

func (eb *EventBus) Start(ctx context.Context) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.running = true
	eb.logger.Info("Event bus started")
	return nil
}

func (eb *EventBus) Stop(ctx context.Context) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.running = false
	eb.logger.Info("Event bus stopped")
	return nil
}

func (eb *EventBus) IsHealthy() bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.running
}

func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.subs[eventType] = append(eb.subs[eventType], handler)
	eb.logger.Debug("Subscribed", zap.String("type", eventType), zap.Int("handlers", len(eb.subs[eventType])))
}

func (eb *EventBus) SubscribeAll(handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.wildcard = append(eb.wildcard, handler)
}

func (eb *EventBus) Publish(ctx context.Context, event Event) error {
	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	start := time.Now()
	eb.mu.RLock()
	handlers := append(eb.subs[event.Type], eb.wildcard...)
	eb.mu.RUnlock()

	eb.metrics.mu.Lock()
	eb.metrics.Published++
	eb.metrics.ByType[event.Type]++
	eb.metrics.BySource[event.Source]++
	eb.metrics.mu.Unlock()

	var firstErr error
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
				eb.logger.Error("Event handler failed", zap.Error(err), zap.String("type", event.Type))
				eb.metrics.mu.Lock()
				eb.metrics.Failed++
				eb.metrics.mu.Unlock()
				if firstErr == nil {
					firstErr = err
				}
			} else {
				eb.metrics.mu.Lock()
				eb.metrics.Delivered++
				eb.metrics.mu.Unlock()
			}
		}(handler)
	}

	wg.Wait()

	latency := time.Since(start).Milliseconds()
	eb.metrics.mu.Lock()
	eb.metrics.LatencySumMs += latency
	eb.metrics.LatencyCount++
	eb.metrics.mu.Unlock()

	return firstErr
}

func (eb *EventBus) PublishAsync(event Event) {
	go func() {
		_ = eb.Publish(context.Background(), event)
	}()
}

func (eb *EventBus) GetMetrics() EventMetricsSnapshot {
	eb.metrics.mu.Lock()
	defer eb.metrics.mu.Unlock()
	return EventMetricsSnapshot{
		Published:     eb.metrics.Published,
		Delivered:     eb.metrics.Delivered,
		Failed:        eb.metrics.Failed,
		ByType:        cloneMap(eb.metrics.ByType),
		BySource:      cloneMap(eb.metrics.BySource),
		AvgLatencyMs:  avg(eb.metrics.LatencySumMs, eb.metrics.LatencyCount),
	}
}

type EventMetricsSnapshot struct {
	Published     int64
	Delivered     int64
	Failed        int64
	ByType        map[string]int64
	BySource      map[string]int64
	AvgLatencyMs  float64
}

func cloneMap(m map[string]int64) map[string]int64 {
	r := make(map[string]int64, len(m))
	for k, v := range m {
		r[k] = v
	}
	return r
}

func avg(sum, count int64) float64 {
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

func generateEventID() string {
	return "evt_" + time.Now().UTC().Format("20060102T150405.000000") + "_" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}