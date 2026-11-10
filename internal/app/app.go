// Package app is the composition root: it constructs adapters and wires them into use cases.
// Both the API and the worker build their graph here, so they cannot drift apart.
package app

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"movie_booking/internal/adapter/mail"
	"movie_booking/internal/adapter/payment"
	"movie_booking/internal/adapter/postgres"
	"movie_booking/internal/adapter/rating"
	redisadapter "movie_booking/internal/adapter/redis"
	"movie_booking/internal/config"
	"movie_booking/internal/usecase"
)

type Infra struct {
	Log   *slog.Logger
	Pool  *pgxpool.Pool
	Redis *redis.Client
}

type Services struct {
	Catalog      *usecase.CatalogService
	Booking      *usecase.BookingService
	Users        *usecase.UserService
	Ratings      *usecase.RatingService
	Notification *usecase.NotificationService
}

func NewLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

// Connect opens PostgreSQL and Redis. Callers must Close the returned Infra.
func Connect(ctx context.Context, cfg *config.Config, log *slog.Logger) (*Infra, error) {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		return nil, err
	}
	rdb, err := redisadapter.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Infra{Log: log, Pool: pool, Redis: rdb}, nil
}

func (i *Infra) Close() {
	i.Pool.Close()
	_ = i.Redis.Close()
}

// Readiness returns the dependencies reported by /health.
func (i *Infra) Readiness() map[string]usecase.Readiness {
	return map[string]usecase.Readiness{
		"postgres": i.Pool,
		"redis":    redisadapter.Pinger{C: i.Redis},
	}
}

// BuildServices wires every use case from the infrastructure and configuration.
func BuildServices(infra *Infra, cfg *config.Config) *Services {
	log := infra.Log
	clock := usecase.SystemClock{}

	movies := postgres.NewMovieRepo(infra.Pool)
	cinemas := postgres.NewCinemaRepo(infra.Pool)
	showtimes := postgres.NewShowtimeRepo(infra.Pool)
	bookings := postgres.NewBookingRepo(infra.Pool, cfg.BookingEventsTopic)
	users := postgres.NewUserRepo(infra.Pool)
	ratingStore := postgres.NewRatingRepo(infra.Pool)

	gate := redisadapter.NewBreakerGate(redisadapter.NewSeatGate(infra.Redis), 3, 5*time.Second)
	ratingCache := redisadapter.NewRatingCache(infra.Redis)
	dedupe := redisadapter.NewDedupe(infra.Redis)

	gateway := payment.NewMockGateway(cfg.PaymentDeclineAll)

	ratings := usecase.NewRatingService(ratingCache, ratingStore, partnerProviders(cfg, log), clock,
		usecase.RatingConfig{
			CacheTTL:       cfg.RatingCacheTTL,
			SyncInterval:   cfg.RatingSyncInterval,
			PartnerTimeout: cfg.PartnerTimeout,
			ReadBudget:     cfg.RatingReadBudget,
		}, log)

	return &Services{
		Catalog: usecase.NewCatalogService(movies, cinemas, showtimes, ratings, clock),
		Booking: usecase.NewBookingService(bookings, showtimes, cinemas, gate, gateway, clock, usecase.BookingConfig{
			HoldTTL:      cfg.SeatHoldTTL,
			MaxSeats:     cfg.MaxSeatsPerBooking,
			Currency:     cfg.Currency,
			SeatKeepTail: cfg.SeatKeepTail,
		}, log),
		Users:        usecase.NewUserService(users, clock),
		Ratings:      ratings,
		Notification: usecase.NewNotificationService(dedupe, mail.NewTicketComposer(cfg.Currency), mail.LogMailer{Log: log}, cfg.NotificationTTL, log),
	}
}

// partnerProviders returns the rating mock. Real partner clients are out of scope for now.
func partnerProviders(_ *config.Config, log *slog.Logger) []usecase.RatingProvider {
	return []usecase.RatingProvider{rating.NewMockPartner(log)}
}
