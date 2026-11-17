package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	appconfig "movie-booking/internal/config"
)

func main() {
	config := appconfig.Load()
	logger := newLogger(config.LogLevel)
	slog.SetDefault(logger)

	if len(os.Args) != 2 {
		logger.Error("worker_type_required", "usage", workerUsage())
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "email":
		err = runEmailWorker(ctx, config, logger)
	case "hold-expiry":
		err = runHoldExpiryWorker(ctx, config, logger)
	case "rating-updater":
		err = runRatingUpdaterWorker(ctx, config, logger)
	default:
		err = fmt.Errorf("unknown worker type %q; %s", os.Args[1], workerUsage())
	}
	if err != nil {
		logger.Error("worker_stopped_with_error", "worker_type", os.Args[1], "error", err)
		os.Exit(1)
	}
}

func workerUsage() string {
	return "workers <email|hold-expiry|rating-updater>"
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
