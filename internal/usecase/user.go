package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"movie_booking/internal/domain"
)

type UserService struct {
	users UserRepository
	clock Clock
}

func NewUserService(users UserRepository, clock Clock) *UserService {
	return &UserService{users: users, clock: clock}
}

func (s *UserService) Register(ctx context.Context, email, fullName string) (*domain.User, error) {
	u, err := domain.NewUser(email, fullName, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}
