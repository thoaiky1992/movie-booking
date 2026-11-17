package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

const defaultRedisURL = "redis://localhost:6379/0"

type Config struct {
	LogLevel slog.Level
	HTTP     HTTP
	Database Database
	Redis    Redis
	Booking  Booking
	Workers  Workers
}

type HTTP struct {
	Port            int
	IdleTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
}

type Database struct {
	Host               string
	Port               string
	Name               string
	Username           string
	Password           string
	Schema             string
	MaxOpenConnections int
	MaxIdleConnections int
	MaxLifetime        time.Duration
	MaxIdleTime        time.Duration
}

type Redis struct {
	URL string
}

type Booking struct {
	SeatHoldTTL time.Duration
}

type Workers struct {
	AsynqConcurrency      int
	AsynqShutdownTimeout  time.Duration
	HoldExpiryInterval    time.Duration
	HoldExpiryBatchSize   int
	RatingUpdaterInterval time.Duration
}

func Load() Config {
	httpConfig := DefaultHTTP()
	httpConfig.Port = positiveIntFromEnv("PORT", httpConfig.Port)
	httpConfig.ShutdownTimeout = durationFromSeconds(
		"HTTP_SHUTDOWN_TIMEOUT_SECONDS",
		httpConfig.ShutdownTimeout,
	)
	httpConfig.AllowedOrigins = csvFromEnv("CORS_ALLOWED_ORIGINS", httpConfig.AllowedOrigins)

	database := Database{
		Host:               stringFromEnv("DB_HOST", "localhost"),
		Port:               stringFromEnv("DB_PORT", "5432"),
		Name:               stringFromEnv("DB_DATABASE", "movie"),
		Username:           stringFromEnv("DB_USERNAME", "movie"),
		Password:           stringFromEnv("DB_PASSWORD", "movie"),
		Schema:             stringFromEnv("DB_SCHEMA", "public"),
		MaxOpenConnections: positiveIntFromEnv("DB_MAX_OPEN_CONNS", 20),
		MaxIdleConnections: positiveIntFromEnv("DB_MAX_IDLE_CONNS", 10),
		MaxLifetime:        durationFromMinutes("DB_CONN_MAX_LIFETIME_MINUTES", 30*time.Minute),
		MaxIdleTime:        durationFromMinutes("DB_CONN_MAX_IDLE_TIME_MINUTES", 5*time.Minute),
	}
	if database.MaxIdleConnections > database.MaxOpenConnections {
		database.MaxIdleConnections = database.MaxOpenConnections
	}

	return Config{
		LogLevel: logLevelFromEnv("LOG_LEVEL"),
		HTTP:     httpConfig,
		Database: database,
		Redis: Redis{
			URL: stringFromEnv("REDIS_URL", defaultRedisURL),
		},
		Booking: Booking{
			SeatHoldTTL: durationFromSeconds("SEAT_HOLD_TTL_SECONDS", 15*time.Minute),
		},
		Workers: Workers{
			AsynqConcurrency:      positiveIntFromEnv("ASYNQ_CONCURRENCY", 5),
			AsynqShutdownTimeout:  durationFromSeconds("ASYNQ_SHUTDOWN_TIMEOUT_SECONDS", 10*time.Second),
			HoldExpiryInterval:    durationFromSeconds("HOLD_EXPIRY_INTERVAL_SECONDS", 5*time.Second),
			HoldExpiryBatchSize:   positiveIntFromEnv("HOLD_EXPIRY_BATCH_SIZE", 100),
			RatingUpdaterInterval: durationFromSeconds("RATING_UPDATE_INTERVAL_SECONDS", 30*time.Second),
		},
	}
}

func DefaultHTTP() HTTP {
	return HTTP{
		Port:            8080,
		IdleTimeout:     time.Minute,
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		AllowedOrigins:  []string{"http://localhost:5173"},
	}
}

func stringFromEnv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func csvFromEnv(name string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return append([]string(nil), fallback...)
	}

	values := make([]string, 0)
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	if len(values) == 0 {
		return append([]string(nil), fallback...)
	}
	return values
}

func durationFromSeconds(name string, fallback time.Duration) time.Duration {
	return time.Duration(positiveIntFromEnv(name, int(fallback/time.Second))) * time.Second
}

func durationFromMinutes(name string, fallback time.Duration) time.Duration {
	return time.Duration(positiveIntFromEnv(name, int(fallback/time.Minute))) * time.Minute
}

func positiveIntFromEnv(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func logLevelFromEnv(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
