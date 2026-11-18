package cmd

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"movie-booking/internal/server"
	appconfig "movie-booking/shared/config"
)

func newServerCommand(config appconfig.Config, logger *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the HTTP API server",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runServer(command.Context(), config, logger)
		},
	}
}

func runServer(ctx context.Context, config appconfig.Config, logger *slog.Logger) error {
	gin.SetMode(gin.ReleaseMode)
	return server.Run(ctx, config, logger)
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
