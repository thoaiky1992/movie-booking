package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"movie-booking/internal/notification"
)

func runEmailWorker(ctx context.Context, logger *slog.Logger) error {
	redisOption, err := asynq.ParseRedisURI(configuredRedisURL())
	if err != nil {
		return fmt.Errorf("parse Asynq Redis configuration: %w", err)
	}

	handler := notification.NewBookingConfirmationHandler(notification.NewMockMailer(logger))
	mux := asynq.NewServeMux()
	mux.HandleFunc(notification.BookingConfirmationTaskType, handler.ProcessTask)
	concurrency := intFromEnv("ASYNQ_CONCURRENCY", 5)
	server := asynq.NewServer(redisOption, asynq.Config{
		Concurrency:     concurrency,
		Queues:          map[string]int{notification.EmailQueue: 1},
		ShutdownTimeout: 10 * time.Second,
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
