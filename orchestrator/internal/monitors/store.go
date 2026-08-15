// Package monitors implements standing claims: a user registers a scientific
// claim once, and the platform re-evaluates it on a schedule through the same
// grounded Aletheia path as an ad-hoc query, recording every check. A check
// whose verdict changes, or whose confidence moves by ≥0.1, is flagged — the
// signal a literature alert can't give, because keyword alerts don't know
// whether new papers actually changed the answer.
//
// v1 runs the scheduler in-process with results persisted per check;
// re-routing checks through workflow-Engine's durable step path (crash-safe,
// deduped) is the deliberate v2 hardening, not an accident of scope — a
// missed check here re-runs at the next tick and costs one duplicate Claude
// call at worst, which doesn't yet justify the extra moving parts.
package monitors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("monitor not found")

type Monitor struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Claim           string    `json:"claim"`
	IntervalSeconds int       `json:"interval_seconds"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
}

type Check struct {
	ID              int64           `json:"id"`
	MonitorID       string          `json:"monitor_id"`
	CheckedAt       time.Time       `json:"checked_at"`
	Verdict         string          `json:"verdict"`
	Confidence      float64         `json:"confidence"`
	SignalBreakdown json.RawMessage `json:"signal_breakdown,omitempty"`
	DebateID        string          `json:"debate_id"`
	SourceCount     int             `json:"source_count"`
	Changed         bool            `json:"changed"`
	ChangeNote      string          `json:"change_note,omitempty"`
}

type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates the schema idempotently (same convention as internal/auth).
func NewStore(ctx context.Context, pool *pgxpool.Pool) (*Store, error) {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS monitors (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id TEXT NOT NULL,
			claim TEXT NOT NULL,
			interval_seconds INT NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS monitor_checks (
			id BIGSERIAL PRIMARY KEY,
			monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
			checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			verdict TEXT NOT NULL,
			confidence DOUBLE PRECISION NOT NULL,
			signal_breakdown JSONB,
			debate_id TEXT NOT NULL,
			source_count INT NOT NULL DEFAULT 0,
			changed BOOLEAN NOT NULL DEFAULT false,
			change_note TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_monitor_checks_monitor
			ON monitor_checks (monitor_id, checked_at DESC);
	`)
	if err != nil {
		return nil, fmt.Errorf("monitors schema: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Create(ctx context.Context, userID, claim string, interval time.Duration) (Monitor, error) {
	var m Monitor
	err := s.pool.QueryRow(ctx, `
		INSERT INTO monitors (user_id, claim, interval_seconds)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, claim, interval_seconds, active, created_at
	`, userID, claim, int(interval.Seconds())).Scan(
		&m.ID, &m.UserID, &m.Claim, &m.IntervalSeconds, &m.Active, &m.CreatedAt)
	return m, err
}

func (s *Store) ListByUser(ctx context.Context, userID string) ([]Monitor, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, claim, interval_seconds, active, created_at
		FROM monitors WHERE user_id = $1 AND active ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMonitors(rows)
}

func (s *Store) Get(ctx context.Context, userID, id string) (Monitor, error) {
	var m Monitor
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, claim, interval_seconds, active, created_at
		FROM monitors WHERE id = $1 AND user_id = $2 AND active
	`, id, userID).Scan(&m.ID, &m.UserID, &m.Claim, &m.IntervalSeconds, &m.Active, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Monitor{}, ErrNotFound
	}
	return m, err
}

func (s *Store) Deactivate(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE monitors SET active = false WHERE id = $1 AND user_id = $2 AND active
	`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Due returns active monitors whose most recent check is older than their
// interval (or that have never been checked).
func (s *Store) Due(ctx context.Context) ([]Monitor, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.user_id, m.claim, m.interval_seconds, m.active, m.created_at
		FROM monitors m
		LEFT JOIN LATERAL (
			SELECT checked_at FROM monitor_checks
			WHERE monitor_id = m.id ORDER BY checked_at DESC LIMIT 1
		) last ON true
		WHERE m.active
		  AND (last.checked_at IS NULL
		       OR last.checked_at < now() - make_interval(secs => m.interval_seconds))
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMonitors(rows)
}

func (s *Store) LatestCheck(ctx context.Context, monitorID string) (Check, bool, error) {
	var c Check
	err := s.pool.QueryRow(ctx, `
		SELECT id, monitor_id, checked_at, verdict, confidence, signal_breakdown,
		       debate_id, source_count, changed, change_note
		FROM monitor_checks WHERE monitor_id = $1 ORDER BY checked_at DESC LIMIT 1
	`, monitorID).Scan(&c.ID, &c.MonitorID, &c.CheckedAt, &c.Verdict, &c.Confidence,
		&c.SignalBreakdown, &c.DebateID, &c.SourceCount, &c.Changed, &c.ChangeNote)
	if errors.Is(err, pgx.ErrNoRows) {
		return Check{}, false, nil
	}
	return c, err == nil, err
}

func (s *Store) InsertCheck(ctx context.Context, c Check) (Check, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO monitor_checks
			(monitor_id, verdict, confidence, signal_breakdown, debate_id,
			 source_count, changed, change_note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, checked_at
	`, c.MonitorID, c.Verdict, c.Confidence, c.SignalBreakdown, c.DebateID,
		c.SourceCount, c.Changed, c.ChangeNote).Scan(&c.ID, &c.CheckedAt)
	return c, err
}

// History returns a monitor's checks oldest-first (chart order).
func (s *Store) History(ctx context.Context, userID, monitorID string, limit int) ([]Check, error) {
	if _, err := s.Get(ctx, userID, monitorID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, monitor_id, checked_at, verdict, confidence, signal_breakdown,
		       debate_id, source_count, changed, change_note
		FROM monitor_checks WHERE monitor_id = $1
		ORDER BY checked_at DESC LIMIT $2
	`, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var checks []Check
	for rows.Next() {
		var c Check
		if err := rows.Scan(&c.ID, &c.MonitorID, &c.CheckedAt, &c.Verdict, &c.Confidence,
			&c.SignalBreakdown, &c.DebateID, &c.SourceCount, &c.Changed, &c.ChangeNote); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	// reverse to oldest-first
	for i, j := 0, len(checks)-1; i < j; i, j = i+1, j-1 {
		checks[i], checks[j] = checks[j], checks[i]
	}
	return checks, rows.Err()
}

func scanMonitors(rows pgx.Rows) ([]Monitor, error) {
	var monitors []Monitor
	for rows.Next() {
		var m Monitor
		if err := rows.Scan(&m.ID, &m.UserID, &m.Claim, &m.IntervalSeconds, &m.Active, &m.CreatedAt); err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, rows.Err()
}
