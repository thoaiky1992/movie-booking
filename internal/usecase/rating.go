package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"movie_booking/internal/domain"
)

// RatingView is what the movie detail page renders.
//
//	cache    – served from Redis (fresh)
//	database – Redis miss, served from the durable last-known snapshot
//	unavailable – no data yet; the page still renders without ratings
type RatingView struct {
	Status    string
	Ratings   []domain.RatingValue
	UpdatedAt *time.Time
}

type RatingConfig struct {
	CacheTTL       time.Duration // Redis TTL for each snapshot (6–24h)
	SyncInterval   time.Duration // how often each movie is refreshed from partners
	PartnerTimeout time.Duration // per-partner call timeout (e.g. 500ms)
	ReadBudget     time.Duration // max time the detail page waits for ratings (e.g. 80ms)
}

type RatingService struct {
	cache     RatingCache
	store     RatingStore
	providers []RatingProvider
	clock     Clock
	cfg       RatingConfig
	log       *slog.Logger
}

func NewRatingService(cache RatingCache, store RatingStore, providers []RatingProvider, clock Clock, cfg RatingConfig, log *slog.Logger) *RatingService {
	return &RatingService{cache: cache, store: store, providers: providers, clock: clock, cfg: cfg, log: log}
}

// ForMovie never calls a partner. It reads Redis, then Postgres, under a strict
// time budget, so a slow or failing dependency cannot slow the detail page.
func (s *RatingService) ForMovie(ctx context.Context, movieID uuid.UUID) RatingView {
	// Each tier gets its own slice of the budget, so a slow cache cannot starve
	// the database fallback. Worst case: cache (half) + database (full) < 150ms.
	cctx, cancel := context.WithTimeout(ctx, s.cfg.ReadBudget/2)
	snap, err := s.cache.Get(cctx, movieID)
	cancel()
	if err == nil {
		return viewOf("cache", snap)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		s.log.Debug("rating cache read failed", "movie_id", movieID, "err", err)
	}

	dctx, dcancel := context.WithTimeout(ctx, s.cfg.ReadBudget)
	defer dcancel()
	snap, err = s.store.Get(dctx, movieID)
	if err == nil {
		// Cache-aside: repopulate Redis off the request path.
		go s.warmCache(movieID, *snap)
		return viewOf("database", snap)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		s.log.Debug("rating store read failed", "movie_id", movieID, "err", err)
	}
	return RatingView{Status: "unavailable", Ratings: []domain.RatingValue{}}
}

func (s *RatingService) warmCache(movieID uuid.UUID, snap domain.RatingSnapshot) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := s.cache.Set(ctx, snap, s.cfg.CacheTTL); err != nil {
		s.log.Debug("warm rating cache failed", "movie_id", movieID, "err", err)
	}
}

func viewOf(status string, snap *domain.RatingSnapshot) RatingView {
	updated := snap.SyncedAt
	return RatingView{Status: status, Ratings: snap.Ratings, UpdatedAt: &updated}
}

// SyncStale is run by the worker. It refreshes movies not attempted within
// SyncInterval, calling partners concurrently with a per-call timeout.
func (s *RatingService) SyncStale(ctx context.Context, limit int) (int, error) {
	if len(s.providers) == 0 {
		// Nothing to ask. Do not record attempts, or enabling a partner later would wait a full interval.
		return 0, nil
	}
	now := s.clock.Now()
	movies, err := s.store.ListStale(ctx, now.Add(-s.cfg.SyncInterval), limit)
	if err != nil {
		return 0, fmt.Errorf("list stale movies: %w", err)
	}
	for _, m := range movies {
		if ctx.Err() != nil {
			break
		}
		s.syncMovie(ctx, m, now)
	}
	return len(movies), nil
}

func (s *RatingService) syncMovie(ctx context.Context, m domain.Movie, now time.Time) {
	type result struct {
		values []domain.RatingValue
		err    error
	}
	results := make([]result, len(s.providers))

	var wg sync.WaitGroup
	for i, p := range s.providers {
		wg.Add(1)
		go func(i int, p RatingProvider) {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, s.cfg.PartnerTimeout)
			defer cancel()
			vals, err := p.Fetch(pctx, m)
			results[i] = result{values: vals, err: err}
		}(i, p)
	}
	wg.Wait()

	var fresh []domain.RatingValue
	for i, r := range results {
		if r.err != nil {
			s.log.Warn("rating partner failed", "provider", s.providers[i].Name(), "movie_id", m.ID, "err", r.err)
			continue
		}
		fresh = append(fresh, r.values...)
	}

	prev, prevErr := s.store.Get(ctx, m.ID)
	if len(fresh) == 0 {
		if err := s.store.MarkAttempt(ctx, m.ID, now); err != nil {
			s.log.Error("mark rating attempt failed", "movie_id", m.ID, "err", err)
		}
		// Keep the last known snapshot alive in Redis so pages stay fast.
		if prevErr == nil {
			if err := s.cache.Set(ctx, *prev, s.cfg.CacheTTL); err != nil {
				s.log.Debug("refresh cached rating failed", "movie_id", m.ID, "err", err)
			}
		}
		return
	}

	var previous []domain.RatingValue
	if prevErr == nil {
		previous = prev.Ratings
	}
	snap := domain.RatingSnapshot{
		MovieID:  m.ID,
		Ratings:  domain.MergeRatings(previous, fresh),
		SyncedAt: now,
	}
	if err := s.store.Upsert(ctx, snap); err != nil {
		s.log.Error("store rating snapshot failed", "movie_id", m.ID, "err", err)
		return
	}
	if err := s.cache.Set(ctx, snap, s.cfg.CacheTTL); err != nil {
		s.log.Debug("cache rating snapshot failed", "movie_id", m.ID, "err", err)
	}
}
