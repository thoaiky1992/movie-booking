package application

import (
	"context"
	"time"

	"movie-booking/modules/identity/domain"
)

type AuthenticateUserQuery struct {
	repository UserRepository
	timeout    time.Duration
}

func NewAuthenticateUserQuery(repository UserRepository) *AuthenticateUserQuery {
	return &AuthenticateUserQuery{
		repository: repository,
		timeout:    2 * time.Second,
	}
}

func (s *AuthenticateUserQuery) Authenticate(ctx context.Context, userID int64) (bool, error) {
	id, err := domain.NewUserID(userID)
	if err != nil {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.repository.Exists(ctx, id)
}
