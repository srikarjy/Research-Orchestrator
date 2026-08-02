package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type contextKey string

const (
	UserContextKey    contextKey = "user"
	ClaimsContextKey  contextKey = "claims"
	SessionContextKey contextKey = "session"
)

type User struct {
	ID           string   `json:"id"`
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	Roles        []string `json:"roles"`
	Avatar       string   `json:"avatar,omitempty"`
	Provider     string   `json:"provider,omitempty"`
	PasswordHash string   `json:"-"`
}

type SessionStore interface {
	GetSession(sessionID string) (*Session, error)
	SaveSession(session *Session) error
	DeleteSession(sessionID string) error
	DeleteUserSessions(userID string) error
}

type UserStore interface {
	GetUserByID(userID string) (*User, error)
	GetUserByEmail(email string) (*User, error)
	GetPasswordHash(userID string) (string, error)
	CreateUser(user *User, passwordHash string) error
	UpdateUser(user *User) error
}

func AuthMiddleware(config *AuthConfig, sessionStore SessionStore) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenString, err := ExtractBearerToken(authHeader)
			if err != nil {
				http.Error(w, "Unauthorized: missing or invalid authorization header", http.StatusUnauthorized)
				return
			}

			claims, err := ValidateAccessToken(config, tokenString)
			if err != nil {
				if errors.Is(err, ErrExpiredToken) {
					http.Error(w, "Unauthorized: token expired", http.StatusUnauthorized)
					return
				}
				http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
				return
			}

			if sessionStore != nil && claims.SessionID != "" {
				session, err := sessionStore.GetSession(claims.SessionID)
				if err != nil || session == nil || !session.IsValid() {
					http.Error(w, "Unauthorized: session invalid", http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), SessionContextKey, session)
				r = r.WithContext(ctx)
			}

			user := &User{
				ID:    claims.UserID,
				Email: claims.Email,
				Roles: claims.Roles,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, ClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuthMiddleware(config *AuthConfig) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenString, err := ExtractBearerToken(authHeader)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := ValidateAccessToken(config, tokenString)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			user := &User{
				ID:    claims.UserID,
				Email: claims.Email,
				Roles: claims.Roles,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, ClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(roles ...string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r.Context())
			if user == nil {
				http.Error(w, "Forbidden: authentication required", http.StatusForbidden)
				return
			}

			hasRole := false
			for _, requiredRole := range roles {
				for _, userRole := range user.Roles {
					if userRole == requiredRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}

			if !hasRole {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetUser(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}
	return nil
}

func GetClaims(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(ClaimsContextKey).(*Claims); ok {
		return claims
	}
	return nil
}

func GetSession(ctx context.Context) *Session {
	if session, ok := ctx.Value(SessionContextKey).(*Session); ok {
		return session
	}
	return nil
}

func GetUserID(ctx context.Context) string {
	if user := GetUser(ctx); user != nil {
		return user.ID
	}
	return ""
}

func HasRole(ctx context.Context, role string) bool {
	user := GetUser(ctx)
	if user == nil {
		return false
	}
	for _, r := range user.Roles {
		if r == role {
			return true
		}
	}
	return false
}

type RateLimiter struct {
	requests map[string][]time.Time
	window   time.Duration
	limit    int
	mu       sync.Mutex
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		window:   window,
		limit:    limit,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	requests := rl.requests[key]
	validRequests := make([]time.Time, 0, len(requests))
	for _, t := range requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}

	if len(validRequests) >= rl.limit {
		rl.requests[key] = validRequests
		return false
	}

	validRequests = append(validRequests, now)
	rl.requests[key] = validRequests
	return true
}

func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	for key, requests := range rl.requests {
		validRequests := make([]time.Time, 0, len(requests))
		for _, t := range requests {
			if t.After(cutoff) {
				validRequests = append(validRequests, t)
			}
		}
		if len(validRequests) == 0 {
			delete(rl.requests, key)
		} else {
			rl.requests[key] = validRequests
		}
	}
}

func RateLimitMiddleware(limiter *RateLimiter, keyFunc func(*http.Request) string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if !limiter.Allow(key) {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func IPKeyFunc(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = strings.Split(r.RemoteAddr, ":")[0]
	}
	return "ip:" + ip
}

func UserKeyFunc(r *http.Request) string {
	user := GetUser(r.Context())
	if user != nil {
		return "user:" + user.ID
	}
	return IPKeyFunc(r)
}