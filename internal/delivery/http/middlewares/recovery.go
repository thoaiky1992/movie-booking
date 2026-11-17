package middlewares

import (
	"fmt"
	"log/slog"
	nethttp "net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	logger = loggerOrDefault(logger)

	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("panic: %v", recovered)
				_ = c.Error(err)
				logger.ErrorContext(
					c.Request.Context(),
					"http_panic_recovered",
					"trace_id", httpdelivery.EnsureTraceID(c),
					"error", err,
					"stack", string(debug.Stack()),
				)

				if !c.Writer.Written() {
					httpdelivery.RespondError(
						c,
						nethttp.StatusInternalServerError,
						"INTERNAL_ERROR",
						"an internal error occurred",
					)
				}
				c.Abort()
			}
		}()

		c.Next()
	}
}
