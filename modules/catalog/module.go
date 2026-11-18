package catalog

import (
	"go.uber.org/fx"

	"movie-booking/modules/catalog/application"
	"movie-booking/modules/catalog/infrastructure/persistence"
	cataloghttp "movie-booking/modules/catalog/presentation/http_api"
)

var Module = fx.Module("catalog",
	fx.Provide(
		fx.Annotate(
			persistence.NewCatalogRepository,
			fx.As(new(application.CatalogRepository)),
		),
		fx.Annotate(
			persistence.NewExternalRatingRepository,
			fx.As(new(application.ExternalRatingRepository)),
		),
		application.NewListCinemasQuery,
		application.NewListMoviesByCinemaQuery,
		application.NewGetMovieDetailQuery,
		application.NewListShowtimesQuery,
		application.NewGetSeatMapQuery,
		application.NewUpdateExternalRatingCommand,
		cataloghttp.NewHandler,
	),
)
