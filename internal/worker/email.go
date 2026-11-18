package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"go.uber.org/fx"

	"movie-booking/modules/booking/infrastructure/producer"
	bookingevent "movie-booking/modules/booking/presentation/event"
	appconfig "movie-booking/shared/config"
)

func runEmail(
	lifecycle fx.Lifecycle,
	config appconfig.Config,
	logger *slog.Logger,
	handler *bookingevent.BookingConfirmationHandler,
) error {
	redisOption, err := asynq.ParseRedisURI(config.Redis.URL)
	if err != nil {
		return fmt.Errorf("parse Asynq Redis configuration: %w", err)
	}
	mux := asynq.NewServeMux()
	mux.HandleFunc(producer.BookingConfirmationTaskType, handler.ProcessTask)
	server := asynq.NewServer(redisOption, asynq.Config{
		Concurrency:     config.Workers.AsynqConcurrency,
		Queues:          map[string]int{producer.EmailQueue: 1},
		ShutdownTimeout: config.Workers.AsynqShutdownTimeout,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			logger.ErrorContext(ctx, "email_task_failed", "task_type", task.Type(), "error", err)
		}),
	})
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error { return server.Start(mux) },
		OnStop: func(context.Context) error {
			server.Shutdown()
			return nil
		},
	})
	return nil
}
