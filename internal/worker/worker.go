package worker

import (
	"context"
	"fmt"
	"log/slog"

	"go.uber.org/fx"

	"movie-booking/database"
	"movie-booking/modules"
	appconfig "movie-booking/shared/config"
)

const (
	Email         = "email"
	HoldExpiry    = "hold-expiry"
	RatingUpdater = "rating-updater"
)

func Run(ctx context.Context, kind string, config appconfig.Config, logger *slog.Logger) error {
	var job fx.Option
	switch kind {
	case Email:
		job = fx.Invoke(runEmail)
	case HoldExpiry:
		job = fx.Invoke(runHoldExpiry)
	case RatingUpdater:
		job = fx.Invoke(runRatingUpdater)
	default:
		return fmt.Errorf("unknown worker type %q", kind)
	}

	app := fx.New(
		fx.Supply(config, logger),
		database.Module,
		modules.FeatureModules,
		job,
		fx.NopLogger,
	)
	startCtx, cancelStart := context.WithTimeout(ctx, config.HTTP.ShutdownTimeout)
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		return fmt.Errorf("start %s worker: %w", kind, err)
	}
	logger.Info("worker_started", "worker_type", kind)
	<-ctx.Done()

	stopCtx, cancelStop := context.WithTimeout(context.Background(), config.Workers.AsynqShutdownTimeout)
	defer cancelStop()
	if err := app.Stop(stopCtx); err != nil {
		return fmt.Errorf("stop %s worker: %w", kind, err)
	}
	logger.Info("worker_stopped", "worker_type", kind)
	return nil
}
