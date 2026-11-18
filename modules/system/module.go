package system

import (
	"go.uber.org/fx"

	"movie-booking/database"
	"movie-booking/modules/system/application"
	systemhttp "movie-booking/modules/system/presentation/http_api"
)

var Module = fx.Module("system", fx.Provide(
	func(db *database.Database) application.HealthChecker { return db },
	systemhttp.NewHandler,
))
