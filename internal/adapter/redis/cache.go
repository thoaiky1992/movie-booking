package redisadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

// RatingCache stores rating snapshots as JSON under rating:movie:<id>.
type RatingCache struct{ client *redis.Client }

func NewRatingCache(c *redis.Client) *RatingCache { return &RatingCache{client: c} }

var _ usecase.RatingCache = (*RatingCache)(nil)

type cachedSnapshot struct {
	Ratings  []domain.RatingValue `json:"ratings"`
	SyncedAt time.Time            `json:"synced_at"`
}

func ratingKey(id uuid.UUID) string { return "rating:movie:" + id.String() }

func (c *RatingCache) Get(ctx context.Context, movieID uuid.UUID) (*domain.RatingSnapshot, error) {
	raw, err := c.client.Get(ctx, ratingKey(movieID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get rating cache: %w", err)
	}
	var cs cachedSnapshot
	if err := json.Unmarshal(raw, &cs); err != nil {
		return nil, fmt.Errorf("decode rating cache: %w", err)
	}
	return &domain.RatingSnapshot{MovieID: movieID, Ratings: cs.Ratings, SyncedAt: cs.SyncedAt}, nil
}

func (c *RatingCache) Set(ctx context.Context, snap domain.RatingSnapshot, ttl time.Duration) error {
	raw, err := json.Marshal(cachedSnapshot{Ratings: snap.Ratings, SyncedAt: snap.SyncedAt})
	if err != nil {
		return fmt.Errorf("encode rating cache: %w", err)
	}
	return c.client.Set(ctx, ratingKey(snap.MovieID), raw, ttl).Err()
}

// Dedupe implements at-most-once claims with SET NX.
type Dedupe struct{ client *redis.Client }

func NewDedupe(c *redis.Client) *Dedupe { return &Dedupe{client: c} }

var _ usecase.Dedupe = (*Dedupe)(nil)

func (d *Dedupe) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := d.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("claim key: %w", err)
	}
	return ok, nil
}

func (d *Dedupe) Release(ctx context.Context, key string) error {
	return d.client.Del(ctx, key).Err()
}
