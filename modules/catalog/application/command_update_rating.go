package application

import (
	"context"
)

type UpdateExternalRatingCommand struct {
	repository ExternalRatingRepository
}

func NewUpdateExternalRatingCommand(repository ExternalRatingRepository) *UpdateExternalRatingCommand {
	return &UpdateExternalRatingCommand{repository: repository}
}

func (s *UpdateExternalRatingCommand) Execute(ctx context.Context) (int64, error) {
	return s.repository.RandomizeOne(ctx)
}
