package application

import (
	"context"
	"fmt"
	"time"

	"movie-booking/modules/catalog/domain"
)

type GetMovieDetailQuery struct {
	repository CatalogRepository
	timeout    time.Duration
}

func NewGetMovieDetailQuery(repository CatalogRepository) *GetMovieDetailQuery {
	return &GetMovieDetailQuery{repository: repository, timeout: 3 * time.Second}
}

func (q *GetMovieDetailQuery) Execute(ctx context.Context, movieID int64) (*domain.MovieDetail, error) {
	id, err := domain.NewMovieID(movieID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	result, err := q.repository.GetMovieDetail(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get movie detail: %w", err)
	}
	return result, nil
}
