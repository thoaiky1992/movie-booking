package worker

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	bookingapplication "movie-booking/modules/booking/application"
	bookingevent "movie-booking/modules/booking/presentation/event"
	appconfig "movie-booking/shared/config"
)

func runHoldExpiry(
	lifecycle fx.Lifecycle,
	config appconfig.Config,
	logger *slog.Logger,
	useCase *bookingapplication.ExpirePendingHoldsCommand,
) {
	worker := bookingevent.NewHoldExpiryWorker(
		useCase,
		logger,
		config.Workers.HoldExpiryInterval,
		config.Workers.HoldExpiryBatchSize,
	)
	var cancel context.CancelFunc
	done := make(chan struct{})
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
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
