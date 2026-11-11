package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	nethttp "net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	TraceIDHeader = "X-Trace-ID"
	traceIDKey    = "trace_id"
)

type traceIDContextKey struct{}

var fallbackTraceSequence atomic.Uint64

// TraceIDMiddleware accepts a caller-provided trace ID or creates one, then
// makes it available to handlers, logs, downstream calls, and the response.
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		EnsureTraceID(c)
		c.Next()
	}
}

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
		traceID = newTraceID()
	}

	c.Set(traceIDKey, traceID)
	c.Header(TraceIDHeader, traceID)
	c.Request = c.Request.WithContext(context.WithValue(
		c.Request.Context(),
		traceIDContextKey{},
		traceID,
	))
	return traceID
}

func TraceIDFromContext(ctx context.Context) string {
	traceID, _ := ctx.Value(traceIDContextKey{}).(string)
	return traceID
}

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	logger = loggerOrDefault(logger)

	return func(c *gin.Context) {
		startedAt := time.Now()
		traceID := EnsureTraceID(c)
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		attributes := []slog.Attr{
			slog.String("trace_id", traceID),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("latency_ms", time.Since(startedAt).Milliseconds()),
			slog.Int("response_size", c.Writer.Size()),
			slog.String("client_ip", c.ClientIP()),
		}
		if len(c.Errors) > 0 {
			attributes = append(attributes, slog.String("error", c.Errors.String()))
		}

		level := slog.LevelInfo
		switch {
		case c.Writer.Status() >= nethttp.StatusInternalServerError:
			level = slog.LevelError
		case c.Writer.Status() >= nethttp.StatusBadRequest:
			level = slog.LevelWarn
		}
		logger.LogAttrs(c.Request.Context(), level, "http_request", attributes...)
	}
}

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
					"trace_id", EnsureTraceID(c),
					"error", err,
					"stack", string(debug.Stack()),
				)

				if !c.Writer.Written() {
					RespondError(
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

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
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

func newTraceID() string {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err == nil {
		return hex.EncodeToString(random)
	}

	return fmt.Sprintf(
		"%016x%016x",
		uint64(time.Now().UnixNano()),
		fallbackTraceSequence.Add(1),
	)
}
