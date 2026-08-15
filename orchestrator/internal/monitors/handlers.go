package monitors

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/srikarjy/research-orchestrator/orchestrator/internal/auth"
)

// Handlers exposes the monitor CRUD + manual-check endpoints. Mounted under
// /api/v1/monitors, behind the gateway's auth middleware.
type Handlers struct {
	store   *Store
	service *Service
	logger  *zap.Logger
}

func NewHandlers(store *Store, service *Service, logger *zap.Logger) *Handlers {
	return &Handlers{store: store, service: service, logger: logger.Named("monitors")}
}

// userID resolves the caller. With auth disabled (local dev) there is no
// identity; everything belongs to "anonymous" so the feature still works.
func userID(c *gin.Context) string {
	if id, ok := auth.IdentityFrom(c); ok {
		return id.UserID
	}
	return "anonymous"
}

type createMonitorRequest struct {
	Claim string `json:"claim" binding:"required"`
	// IntervalHours between checks; default 24, minimum 1 — every check is a
	// real Claude call.
	IntervalHours int `json:"interval_hours"`
}

func (h *Handlers) Create(c *gin.Context) {
	var req createMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.IntervalHours <= 0 {
		req.IntervalHours = 24
	}
	m, err := h.store.Create(c.Request.Context(), userID(c), req.Claim,
		time.Duration(req.IntervalHours)*time.Hour)
	if err != nil {
		h.logger.Error("create monitor failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handlers) List(c *gin.Context) {
	ms, err := h.store.ListByUser(c.Request.Context(), userID(c))
	if err != nil {
		h.logger.Error("list monitors failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	type row struct {
		Monitor
		Latest *Check `json:"latest,omitempty"`
	}
	out := make([]row, 0, len(ms))
	for _, m := range ms {
		r := row{Monitor: m}
		if latest, ok, err := h.store.LatestCheck(c.Request.Context(), m.ID); err == nil && ok {
			r.Latest = &latest
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, gin.H{"monitors": out})
}

func (h *Handlers) History(c *gin.Context) {
	checks, err := h.store.History(c.Request.Context(), userID(c), c.Param("id"), 200)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "monitor not found"})
		return
	}
	if err != nil {
		h.logger.Error("history failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "history failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"checks": checks})
}

// CheckNow runs a monitor's claim immediately (a real retrieval + Claude
// call, so it sits behind the same rate limit as /api/v1/query).
func (h *Handlers) CheckNow(c *gin.Context) {
	m, err := h.store.Get(c.Request.Context(), userID(c), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "monitor not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	check, err := h.service.RunCheck(c.Request.Context(), m)
	if err != nil {
		h.logger.Error("manual check failed", zap.String("monitor_id", m.ID), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, check)
}

func (h *Handlers) Delete(c *gin.Context) {
	err := h.store.Deactivate(c.Request.Context(), userID(c), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "monitor not found"})
		return
	}
	if err != nil {
		h.logger.Error("delete monitor failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
