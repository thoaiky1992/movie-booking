package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	redisclient "github.com/redis/go-redis/v9"

	appconfig "movie-booking/internal/config"
	httpdelivery "movie-booking/internal/delivery/http"
	asynqinfra "movie-booking/internal/infrastructure/asynq"
	"movie-booking/internal/infrastructure/postgres"
	redisinfra "movie-booking/internal/infrastructure/redis"
	"movie-booking/internal/server"
	"movie-booking/internal/usecase"
)

type application struct {
	httpServer    *http.Server
	database      *postgres.Database
	redisClient   *redisclient.Client
	emailEnqueuer *asynqinfra.BookingConfirmationEnqueuer
}

func run(ctx context.Context, config appconfig.Config, logger *slog.Logger) (err error) {
	app, err := newApplication(config, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, app.close())
	}()

	return server.Run(ctx, app.httpServer, logger, config.HTTP.ShutdownTimeout)
}

func newApplication(config appconfig.Config, logger *slog.Logger) (_ *application, err error) {
	// Initialize the database connection.
	database, err := postgres.New(config.Database)
	if err != nil {
		return nil, err
	}
	app := &application{database: database}
	defer func() {
		if err != nil {
			err = errors.Join(err, app.close())
		}
	}()
	// Initialize the Redis client and seat locker.
	seatLocker := app.configureRedis(config.Redis.URL, logger)

	// Initialize the Asynq client for email notifications.
	app.emailEnqueuer, err = asynqinfra.NewBookingConfirmationEnqueuer(config.Redis.URL)
	if err != nil {
		return nil, err
	}

	// Initialize the catalog service and handler.
	catalogRepository := postgres.NewCatalogRepository(app.database.DB())
	catalogService := usecase.NewCatalogService(catalogRepository)
	catalogHandler := httpdelivery.NewCatalogHandler(catalogService)

	// Initialize the booking service and handler.
	bookingRepository := postgres.NewBookingRepository(app.database.DB())
	bookingService := usecase.NewBookingService(
		bookingRepository,
		seatLocker,
		app.emailEnqueuer,
		logger,
		config.Booking.SeatHoldTTL,
	)
	bookingHandler := httpdelivery.NewBookingHandler(bookingService)

	// Initialize authentication and system handlers.
	userRepository := postgres.NewUserRepository(app.database.DB())
	systemHandler := httpdelivery.NewSystemHandler(app.database)

	app.httpServer = server.New(config.HTTP, server.Dependencies{
		Authenticate:   httpdelivery.AuthMiddleware(userRepository),
		SystemHandler:  systemHandler,
		CatalogHandler: catalogHandler,
		BookingHandler: bookingHandler,
		Logger:         logger,
	})
	return app, nil
}

func (a *application) configureRedis(
	redisURL string,
	logger *slog.Logger,
) *redisinfra.SeatLocker {
	client, err := redisinfra.NewClient(redisURL)
	if err != nil {
		logger.Warn("redis_seat_locker_disabled", "error", err)
		return nil
	}
	a.redisClient = client
	return redisinfra.NewSeatLocker(client)
}

func (a *application) close() error {
	var closeErrors []error
	if a.emailEnqueuer != nil {
		if err := a.emailEnqueuer.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close Asynq client: %w", err))
		}
	}
	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close Redis client: %w", err))
		}
	}
	if a.database != nil {
		if err := a.database.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close database: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}
