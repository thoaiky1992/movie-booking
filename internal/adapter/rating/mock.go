// Package rating provides the rating-partner mock. It returns deterministic
// scores and logs each call, standing in for IMDb, Rotten Tomatoes and TMDB.
package rating

import (
	"context"
	"hash/fnv"
	"log/slog"
	"time"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

// MockPartner returns fixed-looking ratings derived from the movie ID, so the
// same movie always gets the same scores across restarts.
type MockPartner struct {
	log *slog.Logger
}

func NewMockPartner(log *slog.Logger) *MockPartner { return &MockPartner{log: log} }

var _ usecase.RatingProvider = (*MockPartner)(nil)

func (m *MockPartner) Name() string { return "mock-partner" }

func (m *MockPartner) Fetch(_ context.Context, movie domain.Movie) ([]domain.RatingValue, error) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(movie.ID.String()))
	base := float64(h.Sum32()%35)/10 + 60 // 6.0 .. 9.4 on a 10-point scale
	now := time.Now().UTC()

	values := []domain.RatingValue{
		{Source: "IMDb", Value: round1(base), Scale: 10, Votes: 10_000 + int64(h.Sum32()%500_000), FetchedAt: now},
		{Source: "Rotten Tomatoes", Value: float64(int(base * 10)), Scale: 100, FetchedAt: now},
		{Source: "TMDB", Value: round1(base - 0.3), Scale: 10, Votes: 1_000 + int64(h.Sum32()%50_000), FetchedAt: now},
	}
	m.log.Info("rating mock: partner values generated", "movie_id", movie.ID, "title", movie.Title, "sources", len(values))
	return values, nil
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
