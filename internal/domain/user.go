package domain

import "context"

// UserRepository is the read port used to verify that an application user
// exists.
type UserRepository interface {
	Exists(ctx context.Context, userID int64) (bool, error)
}
