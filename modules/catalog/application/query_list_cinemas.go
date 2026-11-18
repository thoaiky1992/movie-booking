package application

import (
	"context"
	"fmt"
	"time"

	"movie-booking/modules/catalog/domain"
)

type ListCinemasQuery struct {
	repository CatalogRepository
	timeout    time.Duration
}

func NewListCinemasQuery(repository CatalogRepository) *ListCinemasQuery {
	return &ListCinemasQuery{repository: repository, timeout: 3 * time.Second}
}

func (q *ListCinemasQuery) Execute(ctx context.Context) ([]domain.Cinema, error) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	result, err := q.repository.ListCinemas(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cinemas: %w", err)
	}
	return result, nil
}
