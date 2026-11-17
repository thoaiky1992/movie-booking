package main

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"movie-booking/internal/server"
)

const (
	defaultRedisURL       = "redis://localhost:6379/0"
	defaultSeatHoldTTL    = 15 * time.Minute
	defaultShutdownPeriod = 5 * time.Second
)

type apiConfig struct {
	http            server.Config
	redisURL        string
	seatHoldTTL     time.Duration
	shutdownTimeout time.Duration
	logLevel        slog.Level
}

func loadConfig() apiConfig {
	httpConfig := server.DefaultConfig()
	httpConfig.Port = positiveIntFromEnv("PORT", httpConfig.Port)

	return apiConfig{
		http:            httpConfig,
		redisURL:        stringFromEnv("REDIS_URL", defaultRedisURL),
		seatHoldTTL:     durationFromSeconds("SEAT_HOLD_TTL_SECONDS", defaultSeatHoldTTL),
		shutdownTimeout: durationFromSeconds("HTTP_SHUTDOWN_TIMEOUT_SECONDS", defaultShutdownPeriod),
		logLevel:        logLevelFromEnv("LOG_LEVEL"),
	}
}

func stringFromEnv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func durationFromSeconds(name string, fallback time.Duration) time.Duration {
	fallbackSeconds := int(fallback / time.Second)
	return time.Duration(positiveIntFromEnv(name, fallbackSeconds)) * time.Second
}

func positiveIntFromEnv(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func logLevelFromEnv(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
