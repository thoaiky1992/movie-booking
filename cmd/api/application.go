package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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

type application struct {
	httpServer    *http.Server
	database      database.Service
	redisClient   *redisclient.Client
	emailEnqueuer *notification.AsynqEnqueuer
}

func run(ctx context.Context, config apiConfig, logger *slog.Logger) (err error) {
	app, err := newApplication(config, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, app.close())
	}()

	return server.Run(ctx, app.httpServer, logger, config.shutdownTimeout)
}

func newApplication(config apiConfig, logger *slog.Logger) (_ *application, err error) {
	app := &application{database: database.New()}
	defer func() {
		if err != nil {
			err = errors.Join(err, app.close())
		}
	}()
	// Initialize the Redis client and seat locker
	seatLocker := app.configureRedis(config.redisURL, logger)

	// Initialize the Asynq client for email notifications
	asynqRedisOption, err := asynq.ParseRedisURI(config.redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse Asynq Redis configuration: %w", err)
	}
	app.emailEnqueuer = notification.NewAsynqEnqueuer(asynqRedisOption)

	// Initialize the database connection
	catalogRepository := postgres.NewCatalogRepository(app.database.DB())
	catalogService := usecase.NewCatalogService(catalogRepository)
	catalogHandler := httpdelivery.NewCatalogHandler(catalogService)

	// Initialize the booking service and handler
	bookingRepository := postgres.NewBookingRepository(app.database.DB())
	bookingService := usecase.NewBookingService(
		bookingRepository,
		seatLocker,
		app.emailEnqueuer,
		logger,
		config.seatHoldTTL,
	)
	bookingHandler := httpdelivery.NewBookingHandler(bookingService)

	// Initialize the user repository and system handler
	userRepository := postgres.NewUserRepository(app.database.DB())
	systemHandler := httpdelivery.NewSystemHandler(app.database)

	app.httpServer = server.New(config.http, server.Dependencies{
		UserRepository: userRepository,
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
) *redisrepository.SeatLocker {
	client, err := redisrepository.NewClient(redisURL)
	if err != nil {
		logger.Warn("redis_seat_locker_disabled", "error", err)
		return nil
	}
	a.redisClient = client
	return redisrepository.NewSeatLocker(client)
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
