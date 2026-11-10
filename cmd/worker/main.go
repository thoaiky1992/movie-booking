package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"movie_booking/internal/adapter/kafka"
	"movie_booking/internal/adapter/postgres"
	"movie_booking/internal/app"
	"movie_booking/internal/config"
	"movie_booking/internal/domain"
	"movie_booking/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := app.NewLogger(cfg.LogLevel).With("component", "worker")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	infra, err := app.Connect(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer infra.Close()

	svc := app.BuildServices(infra, cfg)
	publisher := kafka.NewPublisher(cfg.KafkaBrokers)
	defer publisher.Close()

	relay := postgres.NewOutboxRelay(infra.Pool, publisher, 100)
	consumer := kafka.NewConsumer(
		cfg.KafkaBrokers, cfg.BookingEventsTopic, cfg.NotificationGroup, cfg.NotificationDLQTopic,
		publisher, kafka.RetryPolicy{MaxAttempts: 5, BaseDelay: 500 * time.Millisecond, MaxDelay: 10 * time.Second},
		log.With("job", "notification-consumer"),
	)

	var wg sync.WaitGroup
	spawn := func(name string, interval time.Duration, job func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker.Every(ctx, log, name, interval, job)
		}()
	}

	// Outbox relay: commits -> Kafka, typically within a few hundred ms.
	spawn("outbox-relay", cfg.OutboxInterval, func(ctx context.Context) error {
		_, err := relay.PublishPending(ctx)
		return err
	})
	// Durable backstop for hold TTLs. Redis keys also expire on their own.
	spawn("hold-sweeper", cfg.HoldSweepInterval, func(ctx context.Context) error {
		n, err := svc.Booking.ExpireHolds(ctx, 500)
		if n > 0 {
			log.Info("expired holds", "count", n)
		}
		return err
	})
	spawn("refund-retry", cfg.RefundRetryInterval, func(ctx context.Context) error {
		_, err := svc.Booking.RetryPendingRefunds(ctx, 50)
		return err
	})
	spawn("rating-sync", cfg.RatingSyncInterval, func(ctx context.Context) error {
		_, err := svc.Ratings.SyncStale(ctx, cfg.RatingSyncBatch)
		return err
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := consumer.Run(ctx, func(ctx context.Context, _, value []byte) error {
			var evt domain.BookingEvent
			if err := json.Unmarshal(value, &evt); err != nil {
				// A malformed payload will never succeed; retrying cannot help.
				// Returning nil acknowledges it. A production setup would route it to the DLQ instead.
				log.Error("malformed booking event, dropping", "err", err)
				return nil
			}
			return svc.Notification.Handle(ctx, evt)
		})
		if err != nil {
			log.Error("consumer stopped", "err", err)
		}
	}()

	log.Info("worker started", "brokers", cfg.KafkaBrokers, "topic", cfg.BookingEventsTopic)
	<-ctx.Done()
	log.Info("worker shutting down")
	wg.Wait()
	return nil
}
