package http_api

import (
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	"movie-booking/modules/system/application"
	httpdelivery "movie-booking/shared/httpapi"
)

type Handler struct {
	healthChecker application.HealthChecker
}

func NewHandler(healthChecker application.HealthChecker) *Handler {
	return &Handler{healthChecker: healthChecker}
}

func (h *Handler) Root(c *gin.Context) {
	httpdelivery.RespondSuccess(c, nethttp.StatusOK, gin.H{"message": "Hello World"})
}

func (h *Handler) Health(c *gin.Context) {
	httpdelivery.RespondSuccess(c, nethttp.StatusOK, h.healthChecker.Health())
}
