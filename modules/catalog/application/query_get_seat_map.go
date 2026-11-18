package application

import (
	"context"
	"fmt"
	"time"

	"movie-booking/modules/catalog/domain"
)

type GetSeatMapQuery struct {
	repository CatalogRepository
	timeout    time.Duration
}

func NewGetSeatMapQuery(repository CatalogRepository) *GetSeatMapQuery {
	return &GetSeatMapQuery{repository: repository, timeout: 3 * time.Second}
}

func (q *GetSeatMapQuery) Execute(ctx context.Context, showtimeID int64) (*domain.SeatMap, error) {
	id, err := domain.NewShowtimeID(showtimeID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	result, err := q.repository.GetSeatMap(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get seat map: %w", err)
	}
	return result, nil
}
