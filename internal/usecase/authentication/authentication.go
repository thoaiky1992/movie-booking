package authentication

import (
	"context"
	"time"

	"movie-booking/internal/usecase/interfaces"
)

type AuthenticationService struct {
	repository interfaces.UserRepository
	timeout    time.Duration
}

func NewAuthenticationService(repository interfaces.UserRepository) *AuthenticationService {
	return &AuthenticationService{
		repository: repository,
		timeout:    2 * time.Second,
	}
}

func (s *AuthenticationService) Authenticate(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.repository.Exists(ctx, userID)
}
