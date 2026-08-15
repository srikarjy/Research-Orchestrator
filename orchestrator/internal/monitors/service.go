package monitors

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/srikarjy/research-orchestrator/orchestrator/internal/aletheia"
)

// confidenceDelta is the confidence movement that counts as a change worth
// flagging, alongside any verdict flip.
const confidenceDelta = 0.1

// Querier is the slice of the Aletheia client the service needs; the real
// client satisfies it, tests use a fake.
type Querier interface {
	Query(ctx context.Context, claim string) (*aletheia.DebateResponse, error)
}

type Service struct {
	store   *Store
	querier Querier
	logger  *zap.Logger
}

func NewService(store *Store, querier Querier, logger *zap.Logger) *Service {
	return &Service{store: store, querier: querier, logger: logger.Named("monitors")}
}

// RunCheck evaluates a monitor's claim now and records the check, flagging
// verdict flips and confidence moves ≥ confidenceDelta against the previous
// check.
func (s *Service) RunCheck(ctx context.Context, m Monitor) (Check, error) {
	prev, hasPrev, err := s.store.LatestCheck(ctx, m.ID)
	if err != nil {
		return Check{}, fmt.Errorf("latest check: %w", err)
	}

	resp, err := s.querier.Query(ctx, m.Claim)
	if err != nil {
		return Check{}, fmt.Errorf("aletheia query: %w", err)
	}

	check := Check{
		MonitorID:   m.ID,
		Verdict:     resp.Verdict,
		Confidence:  resp.Confidence,
		DebateID:    resp.DebateID,
		SourceCount: len(resp.Sources),
	}
	if resp.SignalBreakdown != nil {
		raw, err := json.Marshal(resp.SignalBreakdown)
		if err != nil {
			return Check{}, fmt.Errorf("marshal breakdown: %w", err)
		}
		check.SignalBreakdown = raw
	}
	check.Changed, check.ChangeNote = diff(prev, hasPrev, check)

	saved, err := s.store.InsertCheck(ctx, check)
	if err != nil {
		return Check{}, fmt.Errorf("insert check: %w", err)
	}
	if saved.Changed {
		s.logger.Info("monitored claim changed",
			zap.String("monitor_id", m.ID),
			zap.String("claim", m.Claim),
			zap.String("note", saved.ChangeNote))
	}
	return saved, nil
}

// diff reports whether the new check differs meaningfully from the previous
// one. The first check is never "changed" — there is nothing to differ from.
func diff(prev Check, hasPrev bool, next Check) (bool, string) {
	if !hasPrev {
		return false, ""
	}
	if prev.Verdict != next.Verdict {
		return true, fmt.Sprintf("verdict changed: %s -> %s", prev.Verdict, next.Verdict)
	}
	// The epsilon keeps a drop of exactly the threshold (e.g. 0.85 -> 0.75)
	// from slipping under it through float64 representation error.
	if delta := next.Confidence - prev.Confidence; math.Abs(delta) >= confidenceDelta-1e-9 {
		return true, fmt.Sprintf("confidence moved %+.2f (%.2f -> %.2f)", delta, prev.Confidence, next.Confidence)
	}
	return false, ""
}

// Start launches the scheduler loop: every tick, run every due monitor.
// Returns a stop function. Checks run sequentially — each is a real Claude
// call, and a burst of parallel calls on a large monitor backlog is exactly
// the spend spike the per-user rate limit exists to prevent elsewhere.
func (s *Service) Start(ctx context.Context, tick time.Duration) func() {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				due, err := s.store.Due(ctx)
				if err != nil {
					s.logger.Warn("due query failed", zap.Error(err))
					continue
				}
				for _, m := range due {
					if ctx.Err() != nil {
						return
					}
					if _, err := s.RunCheck(ctx, m); err != nil {
						s.logger.Warn("scheduled check failed",
							zap.String("monitor_id", m.ID), zap.Error(err))
					}
				}
			}
		}
	}()
	return cancel
}
