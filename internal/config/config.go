// Package config loads runtime settings from the environment (and .env in development).
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                string
	LogLevel           string
	HTTPAddr           string
	CORSAllowedOrigins []string

	DatabaseURL string
	DBMaxConns  int32

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	KafkaBrokers         []string
	BookingEventsTopic   string
	NotificationDLQTopic string
	NotificationGroup    string
	NotificationTTL      time.Duration

	Currency           string
	SeatHoldTTL        time.Duration
	MaxSeatsPerBooking int
	SeatKeepTail       time.Duration
	PaymentDeclineAll  bool

	PartnerTimeout     time.Duration
	RatingCacheTTL     time.Duration
	RatingSyncInterval time.Duration
	RatingReadBudget   time.Duration

	OutboxInterval      time.Duration
	HoldSweepInterval   time.Duration
	RefundRetryInterval time.Duration
	RatingSyncBatch     int
}

// Load reads configuration and reports every invalid value at once.
func Load() (*Config, error) {
	l := &loader{}
	c := &Config{
		Env:                l.str("APP_ENV", "local"),
		LogLevel:           l.str("LOG_LEVEL", "info"),
		HTTPAddr:           l.str("HTTP_ADDR", ":8080"),
		CORSAllowedOrigins: l.list("CORS_ALLOWED_ORIGINS", ""),

		DatabaseURL: l.str("DATABASE_URL", "postgres://movie:movie@localhost:5432/movie?sslmode=disable"),
		DBMaxConns:  int32(l.integer("DB_MAX_CONNS", 20)),

		RedisAddr:     l.str("REDIS_ADDR", "localhost:6379"),
		RedisPassword: l.str("REDIS_PASSWORD", ""),
		RedisDB:       l.integer("REDIS_DB", 0),

		KafkaBrokers:         l.list("KAFKA_BROKERS", "localhost:9092"),
		BookingEventsTopic:   l.str("KAFKA_BOOKING_TOPIC", "booking.events"),
		NotificationDLQTopic: l.str("KAFKA_NOTIFICATION_DLQ_TOPIC", "notification.dlq"),
		NotificationGroup:    l.str("KAFKA_NOTIFICATION_GROUP", "notification-service"),
		NotificationTTL:      l.duration("NOTIFICATION_DEDUPE_TTL", 7*24*time.Hour),

		Currency:           l.str("CURRENCY", "VND"),
		SeatHoldTTL:        l.duration("SEAT_HOLD_TTL", 10*time.Minute),
		MaxSeatsPerBooking: l.integer("MAX_SEATS_PER_BOOKING", 10),
		SeatKeepTail:       l.duration("SEAT_LOCK_KEEP_AFTER_SHOW", 24*time.Hour),
		PaymentDeclineAll:  l.boolean("MOCK_PAYMENT_DECLINE_ALL", false),

		RatingCacheTTL:     l.duration("RATING_CACHE_TTL", 12*time.Hour),
		RatingSyncInterval: l.duration("RATING_SYNC_INTERVAL", time.Hour),
		RatingReadBudget:   l.duration("RATING_READ_BUDGET", 80*time.Millisecond),
		PartnerTimeout:     l.duration("PARTNER_TIMEOUT", 500*time.Millisecond),

		OutboxInterval:      l.duration("WORKER_OUTBOX_INTERVAL", 200*time.Millisecond),
		HoldSweepInterval:   l.duration("WORKER_HOLD_SWEEP_INTERVAL", 10*time.Second),
		RefundRetryInterval: l.duration("WORKER_REFUND_RETRY_INTERVAL", 30*time.Second),
		RatingSyncBatch:     l.integer("WORKER_RATING_SYNC_BATCH", 50),
	}
	if l.err != nil {
		return nil, l.err
	}
	if c.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if c.SeatHoldTTL < time.Minute {
		return nil, errors.New("SEAT_HOLD_TTL must be at least 1m")
	}
	if c.MaxSeatsPerBooking < 1 {
		return nil, errors.New("MAX_SEATS_PER_BOOKING must be >= 1")
	}
	return c, nil
}

type loader struct{ err error }

func (l *loader) fail(key string, err error) {
	if l.err == nil {
		l.err = fmt.Errorf("invalid %s: %w", key, err)
	}
}

func (l *loader) str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func (l *loader) integer(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.fail(key, err)
		return def
	}
	return n
}

func (l *loader) duration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l.fail(key, err)
		return def
	}
	return d
}

func (l *loader) boolean(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		l.fail(key, err)
		return def
	}
	return b
}

func (l *loader) list(key, def string) []string {
	v := l.str(key, def)
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
