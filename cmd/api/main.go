package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	redisclient "github.com/redis/go-redis/v9"

	"movie-booking/internal/database"
	httpdelivery "movie-booking/internal/delivery/http"
	"movie-booking/internal/notification"
	"movie-booking/internal/repository/postgres"
	redisrepository "movie-booking/internal/repository/redis"
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

	redisClient, seatLocker := newRedis(logger)
	if redisClient != nil {
		defer func() {
			if err := redisClient.Close(); err != nil {
				logger.Error("redis_close_failed", "error", err)
			}
		}()
	}

	bookingRepository := postgres.NewBookingRepository(db.DB())
	asynqRedisOption, err := asynq.ParseRedisURI(configuredRedisURL())
	if err != nil {
		logger.Error("asynq_redis_configuration_invalid", "error", err)
		return
	}
	emailEnqueuer := notification.NewAsynqEnqueuer(asynqRedisOption)
	defer func() {
		if err := emailEnqueuer.Close(); err != nil {
			logger.Error("asynq_client_close_failed", "error", err)
		}
	}()
	holdTTL := durationFromSeconds("SEAT_HOLD_TTL_SECONDS", 15*60)
	bookingService := usecase.NewBookingService(
		bookingRepository,
		seatLocker,
		emailEnqueuer,
		logger,
		holdTTL,
	)
	bookingHandler := httpdelivery.NewBookingHandler(bookingService)
	userRepository := postgres.NewUserRepository(db.DB())
	apiServer := server.NewServer(db, userRepository, catalogHandler, bookingHandler, logger)

	// Create a done channel to signal when the shutdown is complete
	done := make(chan bool, 1)

	// Run graceful shutdown in a separate goroutine
	go gracefulShutdown(apiServer, done)

	slog.Info("http_server_started", "address", apiServer.Addr)
	err = apiServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("http_server_failed", "error", err)
		return
	}

	// Wait for the graceful shutdown to complete
	<-done
	slog.Info("graceful_shutdown_complete")
}

func newRedis(logger *slog.Logger) (*redisclient.Client, *redisrepository.SeatLocker) {
	client, err := redisrepository.NewClient(configuredRedisURL())
	if err != nil {
		logger.Warn("redis_disabled", "error", err)
		return nil, nil
	}
	return client, redisrepository.NewSeatLocker(client)
}

func configuredRedisURL() string {
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		return "redis://localhost:6379/0"
	}
	return redisURL
}

func durationFromSeconds(name string, fallbackSeconds int) time.Duration {
	seconds := intFromEnv(name, fallbackSeconds)
	return time.Duration(seconds) * time.Second
}

func intFromEnv(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
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
