package main

import (
	"context"
	"fmt"
	"log/slog"

	redisclient "github.com/redis/go-redis/v9"

	"movie-booking/internal/database"
	"movie-booking/internal/repository/postgres"
	redisrepository "movie-booking/internal/repository/redis"
	"movie-booking/internal/worker"
)

func runHoldExpiryWorker(ctx context.Context, logger *slog.Logger) error {
	db := database.New()
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("database_close_failed", "error", err)
		}
	}()

	redisClient, seatLocker, err := newRedis()
	if err != nil {
		logger.Warn("redis_disabled", "error", err)
	}
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
	return nil
}

func newRedis() (*redisclient.Client, *redisrepository.SeatLocker, error) {
	client, err := redisrepository.NewClient(configuredRedisURL())
	if err != nil {
		return nil, nil, fmt.Errorf("create Redis client: %w", err)
	}
	return client, redisrepository.NewSeatLocker(client), nil
}
