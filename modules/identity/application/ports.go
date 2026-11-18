package application

import (
	"context"

	"movie-booking/modules/identity/domain"
)

type UserRepository interface {
	Exists(context.Context, domain.UserID) (bool, error)
}
