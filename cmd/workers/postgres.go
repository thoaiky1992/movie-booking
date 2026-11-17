package main

import (
	appconfig "movie-booking/internal/config"
	"movie-booking/internal/infrastructure/postgres"
)

func newPostgresDatabase(cfg appconfig.Database) (*postgres.Database, error) {
	return postgres.New(postgres.Config{
		Host:               cfg.Host,
		Port:               cfg.Port,
		Name:               cfg.Name,
		Username:           cfg.Username,
		Password:           cfg.Password,
		Schema:             cfg.Schema,
		MaxOpenConnections: cfg.MaxOpenConnections,
		MaxIdleConnections: cfg.MaxIdleConnections,
		MaxLifetime:        cfg.MaxLifetime,
		MaxIdleTime:        cfg.MaxIdleTime,
	})
}
