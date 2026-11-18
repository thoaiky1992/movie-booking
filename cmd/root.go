package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	appconfig "movie-booking/shared/config"
)

func Execute() error {
	config := appconfig.Load()
	logger := newLogger(config.LogLevel)
	slog.SetDefault(logger)

	root := &cobra.Command{
		Use:           "movie-booking",
		Short:         "Movie Booking service",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return runServer(command.Context(), config, logger)
		},
	}
	ctx, stop := signalContext()
	defer stop()
	root.SetContext(ctx)
	root.AddCommand(
		newServerCommand(config, logger),
		newWorkerCommand(config, logger),
		newMigrationCommand(config),
		newSeedCommand(config),
	)
	return root.Execute()
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
