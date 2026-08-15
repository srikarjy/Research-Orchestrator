package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers exposes the auth HTTP endpoints. Mounted by the gateway under
// /api/v1/auth.
type Handlers struct {
	secret []byte
	store  Store
	logger *zap.Logger
}

func NewHandlers(secret string, store Store, logger *zap.Logger) *Handlers {
	return &Handlers{secret: []byte(secret), store: store, logger: logger.Named("auth")}
}

type credentialsRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register creates a user account and returns a session token.
func (h *Handlers) Register(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.store.CreateUser(c.Request.Context(), strings.ToLower(req.Email), hash)
	if errors.Is(err, ErrDuplicateEmail) {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	if err != nil {
		h.logger.Error("register failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}
	h.issueToken(c, user)
}

// Login verifies credentials and returns a session token.
func (h *Handlers) Login(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.store.GetUserByEmail(c.Request.Context(), strings.ToLower(req.Email))
	// Same rejection for unknown email and wrong password, so login can't be
	// used to probe which emails have accounts.
	if errors.Is(err, ErrNotFound) || (err == nil && !CheckPassword(user.PasswordHash, req.Password)) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		h.logger.Error("login failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}
	h.issueToken(c, user)
}

func (h *Handlers) issueToken(c *gin.Context, user User) {
	token, err := SignToken(h.secret, user.ID, user.Email, time.Now())
	if err != nil {
		h.logger.Error("token signing failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issuance failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  gin.H{"id": user.ID, "email": user.Email},
	})
}

type createKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateKey issues a new API key for the authenticated user. The plaintext
// key appears in this response only and is never recoverable afterwards.
func (h *Handlers) CreateKey(c *gin.Context) {
	id, ok := IdentityFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plaintext, digest, err := NewAPIKey()
	if err != nil {
		h.logger.Error("key generation failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key generation failed"})
		return
	}
	key, err := h.store.CreateAPIKey(c.Request.Context(), id.UserID, req.Name, digest)
	if err != nil {
		h.logger.Error("key creation failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key creation failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":         key.ID,
		"name":       key.Name,
		"api_key":    plaintext,
		"created_at": key.CreatedAt,
		"note":       "store this key now; it cannot be shown again",
	})
}

// ListKeys lists the authenticated user's active API keys (no key material).
func (h *Handlers) ListKeys(c *gin.Context) {
	id, ok := IdentityFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	keys, err := h.store.ListAPIKeys(c.Request.Context(), id.UserID)
	if err != nil {
		h.logger.Error("key listing failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key listing failed"})
		return
	}
	out := make([]gin.H, 0, len(keys))
	for _, k := range keys {
		out = append(out, gin.H{
			"id": k.ID, "name": k.Name, "created_at": k.CreatedAt, "last_used_at": k.LastUsedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"keys": out})
}

// RevokeKey revokes one of the authenticated user's API keys.
func (h *Handlers) RevokeKey(c *gin.Context) {
	id, ok := IdentityFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	err := h.store.RevokeAPIKey(c.Request.Context(), id.UserID, c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	if err != nil {
		h.logger.Error("key revocation failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key revocation failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}
