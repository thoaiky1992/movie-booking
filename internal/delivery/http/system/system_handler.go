package system

import (
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
)

type HealthChecker interface {
	Health() map[string]string
}

type Handler struct {
	healthChecker HealthChecker
}

func NewHandler(healthChecker HealthChecker) *Handler {
	return &Handler{healthChecker: healthChecker}
}

func (h *Handler) Root(c *gin.Context) {
	httpdelivery.RespondSuccess(c, nethttp.StatusOK, gin.H{"message": "Hello World"})
}

func (h *Handler) Health(c *gin.Context) {
	httpdelivery.RespondSuccess(c, nethttp.StatusOK, h.healthChecker.Health())
}
