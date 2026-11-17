package http

import (
	nethttp "net/http"

	"github.com/gin-gonic/gin"
)

type HealthChecker interface {
	Health() map[string]string
}

type SystemHandler struct {
	healthChecker HealthChecker
}

func NewSystemHandler(healthChecker HealthChecker) *SystemHandler {
	return &SystemHandler{healthChecker: healthChecker}
}

func (h *SystemHandler) Root(c *gin.Context) {
	RespondSuccess(c, nethttp.StatusOK, gin.H{"message": "Hello World"})
}

func (h *SystemHandler) Health(c *gin.Context) {
	RespondSuccess(c, nethttp.StatusOK, h.healthChecker.Health())
}
