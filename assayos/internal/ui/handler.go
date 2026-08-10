package ui

import (
	"github.com/gin-gonic/gin"
	"github.com/srikarjy/research-orchestrator/assayos/internal/kernel"
)

type Handler struct {
	platform *kernel.Platform
}

func NewHandler(p *kernel.Platform) *Handler {
	return &Handler{platform: p}
}

func (h *Handler) ServeStatic(c *gin.Context) {
	c.String(404, "UI not available - frontend served separately")
}