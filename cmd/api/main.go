package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	config := loadConfig()
	logger := newLogger(config.logLevel)
	slog.SetDefault(logger)
	gin.SetMode(gin.ReleaseMode)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop() // Restore the default signal behavior so a second signal forces exit.
	}()

	if err := run(ctx, config, logger); err != nil {
		logger.Error("api_stopped_with_error", "error", err)
		os.Exit(1)
	}
}
