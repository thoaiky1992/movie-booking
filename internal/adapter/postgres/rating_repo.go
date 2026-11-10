package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

// RatingRepo is the durable last-known-good store behind the Redis rating cache.
type RatingRepo struct{ pool *pgxpool.Pool }

func NewRatingRepo(pool *pgxpool.Pool) *RatingRepo { return &RatingRepo{pool: pool} }

var _ usecase.RatingStore = (*RatingRepo)(nil)

func (r *RatingRepo) Get(ctx context.Context, movieID uuid.UUID) (*domain.RatingSnapshot, error) {
	var raw []byte
	var syncedAt *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT ratings, synced_at FROM movie_ratings
		WHERE movie_id = $1 AND synced_at IS NOT NULL`, movieID).Scan(&raw, &syncedAt)
	if err != nil {
		return nil, notFoundOr(err, "rating snapshot")
	}
	var values []domain.RatingValue
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode ratings: %w", err)
	}
	return &domain.RatingSnapshot{MovieID: movieID, Ratings: values, SyncedAt: *syncedAt}, nil
}

func (r *RatingRepo) Upsert(ctx context.Context, snap domain.RatingSnapshot) error {
	raw, err := json.Marshal(snap.Ratings)
	if err != nil {
		return fmt.Errorf("encode ratings: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO movie_ratings (movie_id, ratings, synced_at, attempted_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (movie_id) DO UPDATE
		SET ratings = EXCLUDED.ratings, synced_at = EXCLUDED.synced_at, attempted_at = EXCLUDED.attempted_at`,
		snap.MovieID, string(raw), snap.SyncedAt)
	if err != nil {
		return fmt.Errorf("upsert ratings: %w", err)
	}
	return nil
}

func (r *RatingRepo) MarkAttempt(ctx context.Context, movieID uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO movie_ratings (movie_id, ratings, synced_at, attempted_at)
		VALUES ($1, '[]', NULL, $2)
		ON CONFLICT (movie_id) DO UPDATE SET attempted_at = EXCLUDED.attempted_at`, movieID, at)
	if err != nil {
		return fmt.Errorf("mark rating attempt: %w", err)
	}
	return nil
}

// ListStale returns movies with a partner identifier that were not attempted
// since attemptedBefore. Oldest attempts come first, so no movie starves.
func (r *RatingRepo) ListStale(ctx context.Context, attemptedBefore time.Time, limit int) ([]domain.Movie, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.title, m.imdb_id, m.tmdb_id
		FROM movies m
		LEFT JOIN movie_ratings r ON r.movie_id = m.id
		WHERE (m.imdb_id IS NOT NULL OR m.tmdb_id IS NOT NULL)
		  AND (r.attempted_at IS NULL OR r.attempted_at < $1)
		ORDER BY r.attempted_at NULLS FIRST
		LIMIT $2`, attemptedBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("list stale ratings: %w", err)
	}
	defer rows.Close()

	var out []domain.Movie
	for rows.Next() {
		var m domain.Movie
		if err := rows.Scan(&m.ID, &m.Title, &m.IMDbID, &m.TMDbID); err != nil {
			return nil, fmt.Errorf("scan stale movie: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
