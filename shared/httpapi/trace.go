package httpapi

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"movie-booking/shared/observability"
)

const TraceIDHeader = "X-Trace-ID"

const traceIDKey = "trace_id"

// EnsureTraceID returns the request trace ID and creates one when the trace
// middleware is not installed (for example, in a focused handler test).
func EnsureTraceID(c *gin.Context) string {
	if traceID, ok := c.Get(traceIDKey); ok {
		if value, valid := traceID.(string); valid && value != "" {
			return value
		}
	}

	traceID := strings.TrimSpace(c.GetHeader(TraceIDHeader))
	if !validTraceID(traceID) {
		traceID = observability.NewTraceID()
	}

	c.Set(traceIDKey, traceID)
	c.Header(TraceIDHeader, traceID)
	c.Request = c.Request.WithContext(observability.ContextWithTraceID(
		c.Request.Context(),
		traceID,
	))
	return traceID
}

func TraceIDFromContext(ctx context.Context) string {
	return observability.TraceIDFromContext(ctx)
}

func validTraceID(traceID string) bool {
	if traceID == "" || len(traceID) > 128 {
		return false
	}
	for _, character := range traceID {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' {
			continue
		}
		switch character {
		case '-', '_', '.', ':':
			continue
		default:
			return false
		}
	}
	return true
}
