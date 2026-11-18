package database

import (
	"context"
	"database/sql"
	"log/slog"

	"go.uber.org/fx"

	appconfig "movie-booking/shared/config"
)

var Module = fx.Module("database",
	fx.Provide(NewFromConfig),
	fx.Provide(func(database *Database) *sql.DB { return database.DB() }),
)

func NewFromConfig(
	lifecycle fx.Lifecycle,
	config appconfig.Config,
	logger *slog.Logger,
) (*Database, error) {
	database, err := New(Config{
		Host:               config.Database.Host,
		Port:               config.Database.Port,
		Name:               config.Database.Name,
		Username:           config.Database.Username,
		Password:           config.Database.Password,
		Schema:             config.Database.Schema,
		MaxOpenConnections: config.Database.MaxOpenConnections,
		MaxIdleConnections: config.Database.MaxIdleConnections,
		MaxLifetime:        config.Database.MaxLifetime,
		MaxIdleTime:        config.Database.MaxIdleTime,
	})
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		if err := database.Close(); err != nil {
			logger.Error("database_close_failed", "error", err)
			return err
		}
		return nil
	}})
	return database, nil
}
