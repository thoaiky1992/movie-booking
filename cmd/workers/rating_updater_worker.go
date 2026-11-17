package main

import (
	"context"
	"log/slog"

	appconfig "movie-booking/internal/config"
	worker "movie-booking/internal/delivery/worker"
	"movie-booking/internal/infrastructure/postgres"
	catalogusecase "movie-booking/internal/usecase/catalog"
)

func runRatingUpdaterWorker(ctx context.Context, config appconfig.Config, logger *slog.Logger) error {
	db, err := newPostgresDatabase(config.Database)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("database_close_failed", "error", err)
		}
	}()

	interval := config.Workers.RatingUpdaterInterval
	repository := postgres.NewExternalRatingRepository(db.DB())
	ratingService := catalogusecase.NewRatingService(repository)
	ratingUpdater := worker.NewRatingUpdater(ratingService, logger, interval)

	logger.Info("rating_updater_worker_started", "interval", interval.String())
	ratingUpdater.Run(ctx)
	logger.Info("rating_updater_worker_stopped")
	return nil
}
