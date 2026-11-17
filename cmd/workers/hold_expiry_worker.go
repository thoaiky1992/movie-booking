package main

import (
	"context"
	"fmt"
	"log/slog"

	redisclient "github.com/redis/go-redis/v9"

	appconfig "movie-booking/internal/config"
	worker "movie-booking/internal/delivery/worker"
	"movie-booking/internal/infrastructure/postgres"
	redisinfra "movie-booking/internal/infrastructure/redis"
)

func runHoldExpiryWorker(ctx context.Context, config appconfig.Config, logger *slog.Logger) error {
	db, err := postgres.New(config.Database)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("database_close_failed", "error", err)
		}
	}()

	redisClient, seatLocker, err := newRedis(config.Redis.URL)
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
	interval := config.Workers.HoldExpiryInterval
	batchSize := config.Workers.HoldExpiryBatchSize
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

func newRedis(redisURL string) (*redisclient.Client, *redisinfra.SeatLocker, error) {
	client, err := redisinfra.NewClient(redisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("create Redis client: %w", err)
	}
	return client, redisinfra.NewSeatLocker(client), nil
}
