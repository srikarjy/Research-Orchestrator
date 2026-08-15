package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ContextKey is where the middleware stores the authenticated Identity in
// the gin context.
const ContextKey = "auth.identity"

// Middleware bundles the pieces route protection needs.
type Middleware struct {
	secret []byte
	store  Store
	redis  *redis.Client
	logger *zap.Logger
}

// NewMiddleware returns route middleware. secret empty = auth disabled:
// Require passes everything through unauthenticated, and the caller is
// expected to have logged a prominent warning (Enabled tells it to).
func NewMiddleware(secret string, store Store, rdb *redis.Client, logger *zap.Logger) *Middleware {
	return &Middleware{secret: []byte(secret), store: store, redis: rdb, logger: logger.Named("auth")}
}

// Enabled reports whether auth is actually enforced.
func (m *Middleware) Enabled() bool { return len(m.secret) > 0 }

// IdentityFrom returns the authenticated identity, if any.
func IdentityFrom(c *gin.Context) (Identity, bool) {
	v, ok := c.Get(ContextKey)
	if !ok {
		return Identity{}, false
	}
	id, ok := v.(Identity)
	return id, ok
}

// Require rejects requests without a valid JWT or API key. With auth
// disabled it is a no-op passthrough.
func (m *Middleware) Require() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.Enabled() {
			c.Next()
			return
		}
		credential := bearerCredential(c)
		if credential == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing credentials: pass Authorization: Bearer <token or API key>"})
			return
		}

		var id Identity
		var err error
		if IsAPIKey(credential) {
			id, err = m.store.GetIdentityByKeyDigest(c.Request.Context(), HashAPIKey(credential))
		} else {
			id, err = VerifyToken(m.secret, credential)
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.Set(ContextKey, id)
		c.Next()
	}
}

func bearerCredential(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// X-API-Key as a convenience for tools that can't set Authorization.
	return c.GetHeader("X-API-Key")
}

// RateLimit allows at most limit requests per window per identity (falling
// back to client IP when unauthenticated, e.g. the login endpoint itself).
// Backed by a Redis fixed window (INCR + EXPIRE). On a Redis error it fails
// open with a logged warning: for this service, an outage of the limiter
// degrading to unlimited is preferred over the whole API going down —
// revisit if the query endpoint's spend ever outgrows that trade-off.
func (m *Middleware) RateLimit(name string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		who := c.ClientIP()
		if id, ok := IdentityFrom(c); ok {
			who = id.UserID
		}
		key := fmt.Sprintf("ratelimit:%s:%s:%d", name, who, time.Now().Unix()/int64(window.Seconds()))

		count, err := m.redis.Incr(c.Request.Context(), key).Result()
		if err != nil {
			m.logger.Warn("rate limiter unavailable, failing open", zap.Error(err))
			c.Next()
			return
		}
		if count == 1 {
			m.redis.Expire(c.Request.Context(), key, window)
		}
		if count > int64(limit) {
			c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": fmt.Sprintf("rate limit exceeded: %d requests per %s", limit, window),
			})
			return
		}
		c.Next()
	}
}
