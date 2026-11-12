package main

import (
	"context"
	"log/slog"

	"movie-booking/internal/database"
	"movie-booking/internal/repository/postgres"
	"movie-booking/internal/worker"
)

func runRatingUpdaterWorker(ctx context.Context, logger *slog.Logger) error {
	db := database.New()
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("database_close_failed", "error", err)
		}
	}()

	interval := durationFromSeconds("RATING_UPDATE_INTERVAL_SECONDS", 30)
	repository := postgres.NewExternalRatingRepository(db.DB())
	ratingUpdater := worker.NewRatingUpdater(repository, logger, interval)

	logger.Info("rating_updater_worker_started", "interval", interval.String())
	ratingUpdater.Run(ctx)
	logger.Info("rating_updater_worker_stopped")
	return nil
}
