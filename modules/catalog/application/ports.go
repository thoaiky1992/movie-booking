package application

import (
	"context"

	"movie-booking/modules/catalog/domain"
)

type CatalogRepository interface {
	ListCinemas(context.Context) ([]domain.Cinema, error)
	ListMoviesByCinema(context.Context, domain.CinemaID) ([]domain.Movie, error)
	GetMovieDetail(context.Context, domain.MovieID) (*domain.MovieDetail, error)
	ListShowtimesByCinemaAndMovie(context.Context, domain.CinemaID, domain.MovieID) ([]domain.Showtime, error)
	GetSeatMap(context.Context, domain.ShowtimeID) (*domain.SeatMap, error)
}

type ExternalRatingRepository interface {
	RandomizeOne(context.Context) (int64, error)
}
