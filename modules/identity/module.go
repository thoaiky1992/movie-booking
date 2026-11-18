package identity

import (
	"go.uber.org/fx"

	"movie-booking/modules/identity/application"
	"movie-booking/modules/identity/infrastructure/persistence"
)

var Module = fx.Module("identity",
	fx.Provide(
		fx.Annotate(
			persistence.NewUserRepository,
			fx.As(new(application.UserRepository)),
		),
		application.NewAuthenticateUserQuery,
	),
)
