package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"movie-booking/internal/database"
	httpdelivery "movie-booking/internal/delivery/http"
	"movie-booking/internal/repository/postgres"
	"movie-booking/internal/server"
	"movie-booking/internal/usecase"
)

func gracefulShutdown(apiServer *http.Server, done chan bool) {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Listen for the interrupt signal.
	<-ctx.Done()

	slog.Info("server_shutdown_started")
	stop() // Allow Ctrl+C to force shutdown

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		slog.Error("server_shutdown_failed", "error", err)
	}

	slog.Info("server_stopped")

	// Notify the main goroutine that the shutdown is complete
	done <- true
}

func main() {
	logger := newLogger()
	slog.SetDefault(logger)
	gin.SetMode(gin.ReleaseMode)

	db := database.New()
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("database_close_failed", "error", err)
		}
	}()

	catalogRepository := postgres.NewCatalogRepository(db.DB())
	catalogService := usecase.NewCatalogService(catalogRepository)
	catalogHandler := httpdelivery.NewCatalogHandler(catalogService)
	apiServer := server.NewServer(db, catalogHandler, logger)

	// Create a done channel to signal when the shutdown is complete
	done := make(chan bool, 1)

	// Run graceful shutdown in a separate goroutine
	go gracefulShutdown(apiServer, done)

	slog.Info("http_server_started", "address", apiServer.Addr)
	err := apiServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("http_server_failed", "error", err)
		return
	}

	// Wait for the graceful shutdown to complete
	<-done
	slog.Info("graceful_shutdown_complete")
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
