package cmd

import (
	"github.com/spf13/cobra"

	"movie-booking/database"
	appconfig "movie-booking/shared/config"
)

func newMigrationCommand(config appconfig.Config) *cobra.Command {
	var directory string
	command := &cobra.Command{
		Use:       "migration <up|down|status>",
		Short:     "Manage PostgreSQL schema migrations",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"up", "down", "status"},
		RunE: func(command *cobra.Command, args []string) error {
			return database.Migrate(command.Context(), config.Database, directory, args[0])
		},
	}
	command.Flags().StringVar(&directory, "dir", "database/migrations", "migration directory")
	return command
}
