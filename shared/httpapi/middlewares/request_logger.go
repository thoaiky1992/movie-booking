package middlewares

import (
	"log/slog"
	nethttp "net/http"
	"time"

	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/shared/httpapi"
)

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	logger = loggerOrDefault(logger)

	return func(c *gin.Context) {
		startedAt := time.Now()
		traceID := httpdelivery.EnsureTraceID(c)
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
		if userID, ok := httpdelivery.UserIDFromContext(c.Request.Context()); ok {
			attributes = append(attributes, slog.Int64("user_id", userID))
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
