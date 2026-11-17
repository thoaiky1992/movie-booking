package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"movie-booking/internal/config"
)

type Database struct {
	db   *sql.DB
	name string
}

func New(databaseConfig config.Database) (*Database, error) {
	connectionString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		databaseConfig.Username,
		databaseConfig.Password,
		databaseConfig.Host,
		databaseConfig.Port,
		databaseConfig.Name,
		databaseConfig.Schema,
	)
	db, err := sql.Open("pgx", connectionString)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", err)
	}
	db.SetMaxOpenConns(databaseConfig.MaxOpenConnections)
	db.SetMaxIdleConns(databaseConfig.MaxIdleConnections)
	db.SetConnMaxLifetime(databaseConfig.MaxLifetime)
	db.SetConnMaxIdleTime(databaseConfig.MaxIdleTime)

	return &Database{db: db, name: databaseConfig.Name}, nil
}

func (d *Database) DB() *sql.DB {
	return d.db
}

func (d *Database) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stats := make(map[string]string)
	if err := d.db.PingContext(ctx); err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		return stats
	}

	stats["status"] = "up"
	stats["message"] = "It's healthy"
	databaseStats := d.db.Stats()
	stats["open_connections"] = strconv.Itoa(databaseStats.OpenConnections)
	stats["max_open_connections"] = strconv.Itoa(databaseStats.MaxOpenConnections)
	stats["in_use"] = strconv.Itoa(databaseStats.InUse)
	stats["idle"] = strconv.Itoa(databaseStats.Idle)
	stats["wait_count"] = strconv.FormatInt(databaseStats.WaitCount, 10)
	stats["wait_duration"] = databaseStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(databaseStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(databaseStats.MaxLifetimeClosed, 10)

	if databaseStats.OpenConnections > 40 {
		stats["message"] = "The database is experiencing heavy load."
	}
	if databaseStats.WaitCount > 1000 {
		stats["message"] = "The database has a high number of wait events, indicating potential bottlenecks."
	}
	if databaseStats.MaxIdleClosed > int64(databaseStats.OpenConnections)/2 {
		stats["message"] = "Many idle connections are being closed, consider revising the connection pool settings."
	}
	if databaseStats.MaxLifetimeClosed > int64(databaseStats.OpenConnections)/2 {
		stats["message"] = "Many connections are being closed due to max lifetime, consider increasing max lifetime or revising the connection usage pattern."
	}
	return stats
}

func (d *Database) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("close PostgreSQL database %s: %w", d.name, err)
	}
	return nil
}
