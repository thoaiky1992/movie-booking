package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	logger := newLogger()
	slog.SetDefault(logger)

	if len(os.Args) != 2 {
		logger.Error("worker_type_required", "usage", workerUsage())
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "email":
		err = runEmailWorker(ctx, logger)
	case "hold-expiry":
		err = runHoldExpiryWorker(ctx, logger)
	default:
		err = fmt.Errorf("unknown worker type %q; %s", os.Args[1], workerUsage())
	}
	if err != nil {
		logger.Error("worker_stopped_with_error", "worker_type", os.Args[1], "error", err)
		os.Exit(1)
	}
}

func workerUsage() string {
	return "workers <email|hold-expiry>"
}

func configuredRedisURL() string {
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		return "redis://localhost:6379/0"
	}
	return redisURL
}

func durationFromSeconds(name string, fallbackSeconds int) time.Duration {
	return time.Duration(intFromEnv(name, fallbackSeconds)) * time.Second
}

func intFromEnv(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
