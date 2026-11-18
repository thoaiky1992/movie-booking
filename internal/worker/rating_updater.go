package worker

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	catalogapplication "movie-booking/modules/catalog/application"
	catalogevent "movie-booking/modules/catalog/presentation/event"
	appconfig "movie-booking/shared/config"
)

func runRatingUpdater(
	lifecycle fx.Lifecycle,
	config appconfig.Config,
	logger *slog.Logger,
	useCase *catalogapplication.UpdateExternalRatingCommand,
) {
	worker := catalogevent.NewRatingUpdater(
		useCase,
		logger,
		config.Workers.RatingUpdaterInterval,
	)
	var cancel context.CancelFunc
	done := make(chan struct{})
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			var runCtx context.Context
			runCtx, cancel = context.WithCancel(context.Background())
			go func() {
				defer close(done)
				worker.Run(runCtx)
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			select {
			case <-done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}
