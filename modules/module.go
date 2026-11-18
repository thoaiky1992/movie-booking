package modules

import (
	"go.uber.org/fx"

	"movie-booking/modules/booking"
	"movie-booking/modules/catalog"
	"movie-booking/modules/identity"
	"movie-booking/modules/system"
)

var FeatureModules = fx.Options(
	booking.Module,
	catalog.Module,
	identity.Module,
	system.Module,
)
