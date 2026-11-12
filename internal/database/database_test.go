package database

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func mustStartPostgresContainer() (func(context.Context, ...testcontainers.TerminateOption) error, error) {
	var (
		dbName = "database"
		dbPwd  = "password"
		dbUser = "user"
	)

	dbContainer, err := postgres.Run(
		context.Background(),
		"postgres:latest",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPwd),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, err
	}

	database = dbName
	password = dbPwd
	username = dbUser

	dbHost, err := dbContainer.Host(context.Background())
	if err != nil {
		return dbContainer.Terminate, err
	}

	dbPort, err := dbContainer.MappedPort(context.Background(), "5432/tcp")
	if err != nil {
		return dbContainer.Terminate, err
	}

	host = dbHost
	port = dbPort.Port()

	return dbContainer.Terminate, err
}

func TestMain(m *testing.M) {
	teardown, err := mustStartPostgresContainer()
	if err != nil {
		log.Fatalf("could not start postgres container: %v", err)
	}

	m.Run()

	if teardown != nil && teardown(context.Background()) != nil {
		log.Fatalf("could not teardown postgres container: %v", err)
	}
}

func TestNew(t *testing.T) {
	srv := New()
	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

func TestHealth(t *testing.T) {
	srv := New()

	stats := srv.Health()

	if stats["status"] != "up" {
		t.Fatalf("expected status to be up, got %s", stats["status"])
	}

	if _, ok := stats["error"]; ok {
		t.Fatalf("expected error not to be present")
	}

	if stats["message"] != "It's healthy" {
		t.Fatalf("expected message to be 'It's healthy', got %s", stats["message"])
	}
}

func TestClose(t *testing.T) {
	srv := New()

	if srv.Close() != nil {
		t.Fatalf("expected Close() to return nil")
	}
}

func TestDatabasePoolConfigFromEnv(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "24")
	t.Setenv("DB_MAX_IDLE_CONNS", "12")
	t.Setenv("DB_CONN_MAX_LIFETIME_MINUTES", "40")
	t.Setenv("DB_CONN_MAX_IDLE_TIME_MINUTES", "8")

	config := databasePoolConfigFromEnv()
	if config.maxOpenConnections != 24 || config.maxIdleConnections != 12 {
		t.Fatalf(
			"connection limits = open %d idle %d, want open 24 idle 12",
			config.maxOpenConnections,
			config.maxIdleConnections,
		)
	}
	if config.maxLifetime != 40*time.Minute || config.maxIdleTime != 8*time.Minute {
		t.Fatalf("connection durations = lifetime %s idle %s", config.maxLifetime, config.maxIdleTime)
	}
}

func TestDatabasePoolConfigCapsIdleConnections(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "10")
	t.Setenv("DB_MAX_IDLE_CONNS", "25")

	config := databasePoolConfigFromEnv()
	if config.maxIdleConnections != config.maxOpenConnections {
		t.Fatalf(
			"idle connections = %d, want capped at %d",
			config.maxIdleConnections,
			config.maxOpenConnections,
		)
	}
}

func TestDatabasePoolConfigUsesDefaultsForInvalidValues(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "invalid")
	t.Setenv("DB_MAX_IDLE_CONNS", "0")
	t.Setenv("DB_CONN_MAX_LIFETIME_MINUTES", "-1")
	t.Setenv("DB_CONN_MAX_IDLE_TIME_MINUTES", "")

	config := databasePoolConfigFromEnv()
	if config.maxOpenConnections != 20 || config.maxIdleConnections != 10 {
		t.Fatalf(
			"default connection limits = open %d idle %d",
			config.maxOpenConnections,
			config.maxIdleConnections,
		)
	}
	if config.maxLifetime != 30*time.Minute || config.maxIdleTime != 5*time.Minute {
		t.Fatalf("default durations = lifetime %s idle %s", config.maxLifetime, config.maxIdleTime)
	}
}
