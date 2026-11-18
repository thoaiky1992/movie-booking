package application

import (
	"context"
	"fmt"
	"time"

	"movie-booking/modules/catalog/domain"
)

type ListMoviesByCinemaQuery struct {
	repository CatalogRepository
	timeout    time.Duration
}

func NewListMoviesByCinemaQuery(repository CatalogRepository) *ListMoviesByCinemaQuery {
	return &ListMoviesByCinemaQuery{repository: repository, timeout: 3 * time.Second}
}

func (q *ListMoviesByCinemaQuery) Execute(ctx context.Context, cinemaID int64) ([]domain.Movie, error) {
	id, err := domain.NewCinemaID(cinemaID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	result, err := q.repository.ListMoviesByCinema(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list movies by cinema: %w", err)
	}
	return result, nil
}
