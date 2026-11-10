package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

type UserRepo struct{ pool *pgxpool.Pool }

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

var _ usecase.UserRepository = (*UserRepo)(nil)

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, full_name, created_at) VALUES ($1, $2, $3, $4)`,
		u.ID, u.Email, u.FullName, u.CreatedAt)
	if isUniqueViolation(err, "uq_users_email") {
		return fmt.Errorf("%w: email already registered", domain.ErrConflict)
	}
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, `SELECT id, email, full_name, created_at FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Email, &u.FullName, &u.CreatedAt)
	if err != nil {
		return nil, notFoundOr(err, "user")
	}
	return &u, nil
}
