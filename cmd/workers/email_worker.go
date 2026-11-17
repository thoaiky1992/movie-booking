package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	appconfig "movie-booking/internal/config"
	worker "movie-booking/internal/delivery/worker"
	asynqinfra "movie-booking/internal/infrastructure/asynq"
	"movie-booking/internal/infrastructure/mailer"
	bookingusecase "movie-booking/internal/usecase/booking"
)

func runEmailWorker(ctx context.Context, config appconfig.Config, logger *slog.Logger) error {
	redisOption, err := asynq.ParseRedisURI(config.Redis.URL)
	if err != nil {
		return fmt.Errorf("parse Asynq Redis configuration: %w", err)
	}

	confirmationService := bookingusecase.NewBookingConfirmationService(mailer.NewMock(logger))
	handler := worker.NewBookingConfirmationHandler(confirmationService)
	mux := asynq.NewServeMux()
	mux.HandleFunc(asynqinfra.BookingConfirmationTaskType, handler.ProcessTask)
	concurrency := config.Workers.AsynqConcurrency
	server := asynq.NewServer(redisOption, asynq.Config{
		Concurrency:     concurrency,
		Queues:          map[string]int{asynqinfra.EmailQueue: 1},
		ShutdownTimeout: config.Workers.AsynqShutdownTimeout,
		BaseContext: func() context.Context {
			return ctx
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(
			ctx context.Context,
			task *asynq.Task,
			err error,
		) {
			logger.ErrorContext(
				ctx,
				"email_task_failed",
				"task_type", task.Type(),
				"error", err,
			)
		}),
	})
	if err := server.Start(mux); err != nil {
		return fmt.Errorf("start email worker: %w", err)
	}

	logger.Info("email_worker_started", "concurrency", concurrency)
	<-ctx.Done()
	server.Shutdown()
	logger.Info("email_worker_stopped")
	return nil
}
