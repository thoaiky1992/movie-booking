package database

import (
	"context"
	"fmt"

	"github.com/pressly/goose/v3"

	appconfig "movie-booking/shared/config"
)

func Migrate(ctx context.Context, config appconfig.Database, directory, action string) error {
	database, err := New(Config{
		Host: config.Host, Port: config.Port, Name: config.Name,
		Username: config.Username, Password: config.Password, Schema: config.Schema,
		MaxOpenConnections: config.MaxOpenConnections,
		MaxIdleConnections: config.MaxIdleConnections,
		MaxLifetime:        config.MaxLifetime, MaxIdleTime: config.MaxIdleTime,
	})
	if err != nil {
		return err
	}
	defer database.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	switch action {
	case "up":
		err = goose.UpContext(ctx, database.DB(), directory)
	case "down":
		err = goose.DownContext(ctx, database.DB(), directory)
	case "status":
		err = goose.StatusContext(ctx, database.DB(), directory)
	default:
		return fmt.Errorf("unknown migration action %q", action)
	}
	if err != nil {
		return fmt.Errorf("migration %s: %w", action, err)
	}
	return nil
}
