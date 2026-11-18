package application

import (
	"context"
	"fmt"
	"time"

	"movie-booking/modules/catalog/domain"
)

type ListShowtimesQuery struct {
	repository CatalogRepository
	timeout    time.Duration
}

func NewListShowtimesQuery(repository CatalogRepository) *ListShowtimesQuery {
	return &ListShowtimesQuery{repository: repository, timeout: 3 * time.Second}
}

func (q *ListShowtimesQuery) Execute(ctx context.Context, cinemaID, movieID int64) ([]domain.Showtime, error) {
	validatedCinemaID, err := domain.NewCinemaID(cinemaID)
	if err != nil {
		return nil, err
	}
	validatedMovieID, err := domain.NewMovieID(movieID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	result, err := q.repository.ListShowtimesByCinemaAndMovie(ctx, validatedCinemaID, validatedMovieID)
	if err != nil {
		return nil, fmt.Errorf("list showtimes by cinema and movie: %w", err)
	}
	return result, nil
}
