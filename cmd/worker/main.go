package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	"movie-booking/internal/database"
	"movie-booking/internal/repository/postgres"
	redisrepository "movie-booking/internal/repository/redis"
	"movie-booking/internal/worker"
)

func main() {
	logger := newLogger()
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db := database.New()
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("database_close_failed", "error", err)
		}
	}()

	redisClient, seatLocker := newRedis(logger)
	if redisClient != nil {
		defer func() {
			if err := redisClient.Close(); err != nil {
				logger.Error("redis_close_failed", "error", err)
			}
		}()
	}

	bookingRepository := postgres.NewBookingRepository(db.DB())
	interval := durationFromSeconds("HOLD_EXPIRY_INTERVAL_SECONDS", 5)
	batchSize := intFromEnv("HOLD_EXPIRY_BATCH_SIZE", 100)
	expiryWorker := worker.NewHoldExpiryWorker(
		bookingRepository,
		seatLocker,
		logger,
		interval,
		batchSize,
	)

	logger.Info(
		"hold_expiry_worker_started",
		"interval", interval.String(),
		"batch_size", batchSize,
	)
	expiryWorker.Run(ctx)
	logger.Info("hold_expiry_worker_stopped")
}

func newRedis(logger *slog.Logger) (*redisclient.Client, *redisrepository.SeatLocker) {
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	client, err := redisrepository.NewClient(redisURL)
	if err != nil {
		logger.Warn("redis_disabled", "error", err)
		return nil, nil
	}
	return client, redisrepository.NewSeatLocker(client)
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
