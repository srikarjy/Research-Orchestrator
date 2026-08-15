package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a user or key doesn't exist (or is revoked).
var ErrNotFound = errors.New("not found")

// ErrDuplicateEmail is returned when registering an email that already exists.
var ErrDuplicateEmail = errors.New("email already registered")

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type APIKey struct {
	ID        string
	UserID    string
	Name      string
	CreatedAt time.Time
	// LastUsedAt is nil until the key's first authenticated request.
	LastUsedAt *time.Time
}

// Store is the persistence interface the gateway and middleware use;
// pgStore is the real implementation, tests use an in-memory fake.
type Store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	CreateAPIKey(ctx context.Context, userID, name, keyDigest string) (APIKey, error)
	ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error)
	RevokeAPIKey(ctx context.Context, userID, keyID string) error
	// GetIdentityByKeyDigest resolves an API key digest to the owning user,
	// skipping revoked keys, and records the use.
	GetIdentityByKeyDigest(ctx context.Context, digest string) (Identity, error)
}

type pgStore struct {
	pool *pgxpool.Pool
}

// NewStore returns a Postgres-backed Store, creating its schema if needed
// (idempotent — the orchestrator has no migration system, matching how the
// rest of this service manages its tables).
func NewStore(ctx context.Context, pool *pgxpool.Pool) (Store, error) {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS api_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			key_digest TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_used_at TIMESTAMPTZ,
			revoked BOOLEAN NOT NULL DEFAULT false
		);
	`)
	if err != nil {
		return nil, fmt.Errorf("auth schema: %w", err)
	}
	return &pgStore{pool: pool}, nil
}

func (s *pgStore) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash) VALUES ($1, $2)
		ON CONFLICT (email) DO NOTHING
		RETURNING id, email, password_hash, created_at
	`, email, passwordHash).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrDuplicateEmail
	}
	return u, err
}

func (s *pgStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, created_at FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *pgStore) CreateAPIKey(ctx context.Context, userID, name, keyDigest string) (APIKey, error) {
	var k APIKey
	err := s.pool.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, name, key_digest) VALUES ($1, $2, $3)
		RETURNING id, user_id, name, created_at
	`, userID, name, keyDigest).Scan(&k.ID, &k.UserID, &k.Name, &k.CreatedAt)
	return k, err
}

func (s *pgStore) ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, created_at, last_used_at
		FROM api_keys WHERE user_id = $1 AND NOT revoked ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.CreatedAt, &k.LastUsedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *pgStore) RevokeAPIKey(ctx context.Context, userID, keyID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked = true WHERE id = $1 AND user_id = $2 AND NOT revoked
	`, keyID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) GetIdentityByKeyDigest(ctx context.Context, digest string) (Identity, error) {
	var id Identity
	err := s.pool.QueryRow(ctx, `
		UPDATE api_keys SET last_used_at = now()
		FROM users
		WHERE api_keys.key_digest = $1 AND NOT api_keys.revoked
		  AND users.id = api_keys.user_id
		RETURNING users.id, users.email
	`, digest).Scan(&id.UserID, &id.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, ErrNotFound
	}
	id.Via = "api_key"
	return id, err
}
