package main

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadConfigUsesEnvironmentValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("REDIS_URL", " redis://cache:6379/2 ")
	t.Setenv("SEAT_HOLD_TTL_SECONDS", "120")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT_SECONDS", "9")
	t.Setenv("LOG_LEVEL", "warning")

	config := loadConfig()
	if config.http.Port != 9090 {
		t.Fatalf("HTTP port = %d, want 9090", config.http.Port)
	}
	if config.redisURL != "redis://cache:6379/2" {
		t.Fatalf("Redis URL = %q, want %q", config.redisURL, "redis://cache:6379/2")
	}
	if config.seatHoldTTL != 2*time.Minute {
		t.Fatalf("seat hold TTL = %s, want %s", config.seatHoldTTL, 2*time.Minute)
	}
	if config.shutdownTimeout != 9*time.Second {
		t.Fatalf("shutdown timeout = %s, want %s", config.shutdownTimeout, 9*time.Second)
	}
	if config.logLevel != slog.LevelWarn {
		t.Fatalf("log level = %s, want %s", config.logLevel, slog.LevelWarn)
	}
}

func TestLoadConfigFallsBackForInvalidValues(t *testing.T) {
	t.Setenv("PORT", "invalid")
	t.Setenv("REDIS_URL", " ")
	t.Setenv("SEAT_HOLD_TTL_SECONDS", "0")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT_SECONDS", "-1")
	t.Setenv("LOG_LEVEL", "verbose")

	config := loadConfig()
	if config.http.Port != 8080 {
		t.Fatalf("HTTP port = %d, want 8080", config.http.Port)
	}
	if config.redisURL != defaultRedisURL {
		t.Fatalf("Redis URL = %q, want %q", config.redisURL, defaultRedisURL)
	}
	if config.seatHoldTTL != defaultSeatHoldTTL {
		t.Fatalf("seat hold TTL = %s, want %s", config.seatHoldTTL, defaultSeatHoldTTL)
	}
	if config.shutdownTimeout != defaultShutdownPeriod {
		t.Fatalf(
			"shutdown timeout = %s, want %s",
			config.shutdownTimeout,
			defaultShutdownPeriod,
		)
	}
	if config.logLevel != slog.LevelInfo {
		t.Fatalf("log level = %s, want %s", config.logLevel, slog.LevelInfo)
	}
}
