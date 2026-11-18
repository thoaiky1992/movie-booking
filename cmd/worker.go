package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"

	workerbootstrap "movie-booking/internal/worker"
	appconfig "movie-booking/shared/config"
)

func newWorkerCommand(config appconfig.Config, logger *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:       "worker <email|hold-expiry|rating-updater>",
		Short:     "Start a background worker",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{workerbootstrap.Email, workerbootstrap.HoldExpiry, workerbootstrap.RatingUpdater},
		RunE: func(command *cobra.Command, args []string) error {
			return workerbootstrap.Run(command.Context(), args[0], config, logger)
		},
	}
}
