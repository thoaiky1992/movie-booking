package catalog

import (
	"context"

	"movie-booking/internal/usecase/interfaces"
)

type RatingService struct {
	repository interfaces.ExternalRatingRepository
}

func NewRatingService(repository interfaces.ExternalRatingRepository) *RatingService {
	return &RatingService{repository: repository}
}

func (s *RatingService) UpdateOne(ctx context.Context) (int64, error) {
	return s.repository.RandomizeOne(ctx)
}
