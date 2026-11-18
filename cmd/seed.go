package cmd

import (
	"github.com/spf13/cobra"

	"movie-booking/database"
	appconfig "movie-booking/shared/config"
)

// Seed data is versioned as the second migration so fresh environments and
// explicit seed runs stay deterministic.
func newSeedCommand(config appconfig.Config) *cobra.Command {
	var directory string
	command := &cobra.Command{
		Use:   "seed",
		Short: "Apply versioned development seed data",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return database.Migrate(command.Context(), config.Database, directory, "up")
		},
	}
	command.Flags().StringVar(&directory, "dir", "database/migrations", "migration directory")
	return command
}
