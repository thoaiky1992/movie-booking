package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"movie_booking/internal/domain"
)

type CatalogService struct {
	movies    MovieRepository
	cinemas   CinemaRepository
	showtimes ShowtimeRepository
	ratings   *RatingService
	clock     Clock
}

func NewCatalogService(movies MovieRepository, cinemas CinemaRepository, showtimes ShowtimeRepository, ratings *RatingService, clock Clock) *CatalogService {
	return &CatalogService{movies: movies, cinemas: cinemas, showtimes: showtimes, ratings: ratings, clock: clock}
}

func (s *CatalogService) ListMovies(ctx context.Context, f MovieFilter) (Paged[domain.Movie], error) {
	items, total, err := s.movies.List(ctx, f)
	if err != nil {
		return Paged[domain.Movie]{}, fmt.Errorf("list movies: %w", err)
	}
	return Paged[domain.Movie]{Items: items, Total: total, Page: f.Page}, nil
}

// MovieDetail is served on the hot path: the ratings lookup never calls a partner.
type MovieDetail struct {
	Movie   domain.Movie
	Ratings RatingView
}

func (s *CatalogService) GetMovie(ctx context.Context, id uuid.UUID) (*MovieDetail, error) {
	m, err := s.movies.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &MovieDetail{Movie: *m, Ratings: s.ratings.ForMovie(ctx, id)}, nil
}

func (s *CatalogService) ListCinemas(ctx context.Context, p Page) (Paged[domain.Cinema], error) {
	items, total, err := s.cinemas.List(ctx, p)
	if err != nil {
		return Paged[domain.Cinema]{}, fmt.Errorf("list cinemas: %w", err)
	}
	return Paged[domain.Cinema]{Items: items, Total: total, Page: p}, nil
}

func (s *CatalogService) ListHalls(ctx context.Context, cinemaID uuid.UUID) ([]domain.Hall, error) {
	if _, err := s.cinemas.GetByID(ctx, cinemaID); err != nil {
		return nil, err
	}
	return s.cinemas.ListHalls(ctx, cinemaID)
}

func (s *CatalogService) ListShowtimes(ctx context.Context, f ShowtimeFilter) (Paged[domain.ShowtimeView], error) {
	if f.From != nil && f.To != nil && !f.To.After(*f.From) {
		return Paged[domain.ShowtimeView]{}, fmt.Errorf("%w: 'to' must be after 'from'", domain.ErrInvalidInput)
	}
	items, total, err := s.showtimes.List(ctx, f)
	if err != nil {
		return Paged[domain.ShowtimeView]{}, fmt.Errorf("list showtimes: %w", err)
	}
	return Paged[domain.ShowtimeView]{Items: items, Total: total, Page: f.Page}, nil
}

type SeatMapView struct {
	Showtime domain.Showtime
	Seats    []domain.SeatStatus
}

func (s *CatalogService) SeatMap(ctx context.Context, showtimeID uuid.UUID) (*SeatMapView, error) {
	show, err := s.showtimes.GetByID(ctx, showtimeID)
	if err != nil {
		return nil, err
	}
	seats, err := s.showtimes.SeatMap(ctx, showtimeID, s.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("load seat map: %w", err)
	}
	return &SeatMapView{Showtime: *show, Seats: seats}, nil
}
