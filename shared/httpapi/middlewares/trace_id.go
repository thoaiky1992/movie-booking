package middlewares

import (
	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/shared/httpapi"
)

// TraceID accepts a caller-provided trace ID or creates one, then makes it
// available to handlers, logs, downstream calls, and the response.
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		httpdelivery.EnsureTraceID(c)
		c.Next()
	}
}
