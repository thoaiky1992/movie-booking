// Package redisadapter implements the seat gate, rating cache and dedupe ports on Redis.
package redisadapter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient returns a client with tight timeouts. Redis sits on hot paths, so
// a slow Redis must fail fast and fall back, not queue requests behind it.
func NewClient(ctx context.Context, addr, password string, db int) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  300 * time.Millisecond,
		ReadTimeout:  150 * time.Millisecond,
		WriteTimeout: 150 * time.Millisecond,
		PoolTimeout:  150 * time.Millisecond,
		PoolSize:     50,
		MaxRetries:   -1, // fail fast; callers have fallbacks, and retries only add latency
	})
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return c, nil
}

// Pinger adapts the client to the readiness port.
type Pinger struct{ C *redis.Client }

func (p Pinger) Ping(ctx context.Context) error { return p.C.Ping(ctx).Err() }
