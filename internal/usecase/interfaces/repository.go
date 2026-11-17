package interfaces

import (
	"context"
	"time"

	"movie-booking/internal/domain"
)

type BookingRepository interface {
	HoldSeats(
		ctx context.Context,
		userID int64,
		showtimeID int64,
		seatIDs []int64,
		holdToken string,
		ttl time.Duration,
	) (*domain.SeatHold, error)
	ConfirmBooking(ctx context.Context, userID, bookingID int64) (*domain.ConfirmedBooking, error)
	CancelBooking(ctx context.Context, userID, bookingID int64) (*domain.CancelledBooking, error)
}

type CatalogRepository interface {
	ListCinemas(ctx context.Context) ([]domain.Cinema, error)
	ListMoviesByCinema(ctx context.Context, cinemaID int64) ([]domain.Movie, error)
	GetMovieDetail(ctx context.Context, movieID int64) (*domain.MovieDetail, error)
	ListShowtimesByCinemaAndMovie(ctx context.Context, cinemaID, movieID int64) ([]domain.Showtime, error)
	GetSeatMap(ctx context.Context, showtimeID int64) (*domain.SeatMap, error)
}

type PendingHoldRepository interface {
	ExpirePendingHolds(ctx context.Context, batchSize int) ([]domain.ReleasedSeat, error)
}

type ExternalRatingRepository interface {
	RandomizeOne(ctx context.Context) (int64, error)
}

type UserRepository interface {
	Exists(ctx context.Context, userID int64) (bool, error)
}
