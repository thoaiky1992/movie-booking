package server

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestNewAppliesConfig(t *testing.T) {
	config := Config{
		Port:           9090,
		IdleTimeout:    2 * time.Minute,
		ReadTimeout:    12 * time.Second,
		WriteTimeout:   45 * time.Second,
		AllowedOrigins: []string{"https://example.com"},
	}

	httpServer := New(config, Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if httpServer.Addr != ":9090" {
		t.Fatalf("address = %q, want %q", httpServer.Addr, ":9090")
	}
	if httpServer.IdleTimeout != config.IdleTimeout {
		t.Fatalf("idle timeout = %s, want %s", httpServer.IdleTimeout, config.IdleTimeout)
	}
	if httpServer.ReadTimeout != config.ReadTimeout {
		t.Fatalf("read timeout = %s, want %s", httpServer.ReadTimeout, config.ReadTimeout)
	}
	if httpServer.WriteTimeout != config.WriteTimeout {
		t.Fatalf("write timeout = %s, want %s", httpServer.WriteTimeout, config.WriteTimeout)
	}
}

func TestConfigWithDefaults(t *testing.T) {
	config := (Config{}).withDefaults()
	defaults := DefaultConfig()

	if config.Port != defaults.Port ||
		config.IdleTimeout != defaults.IdleTimeout ||
		config.ReadTimeout != defaults.ReadTimeout ||
		config.WriteTimeout != defaults.WriteTimeout {
		t.Fatalf("config = %+v, want defaults %+v", config, defaults)
	}
	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "http://localhost:5173" {
		t.Fatalf("allowed origins = %v, want local frontend", config.AllowedOrigins)
	}
}
