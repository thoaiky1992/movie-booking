package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"movie-booking/modules/identity/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Exists(ctx context.Context, userID domain.UserID) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`,
		userID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check user existence: %w", err)
	}
	return exists, nil
}
