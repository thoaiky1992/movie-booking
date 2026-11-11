package usecase

import (
	"context"
	"fmt"
	"time"

	"movie-booking/internal/domain"
)

type CatalogService struct {
	repository domain.CatalogRepository
	timeout    time.Duration
}

func NewCatalogService(repository domain.CatalogRepository) *CatalogService {
	return &CatalogService{
		repository: repository,
		timeout:    3 * time.Second,
	}
}

func (s *CatalogService) ListCinemas(ctx context.Context) ([]domain.Cinema, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cinemas, err := s.repository.ListCinemas(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cinemas: %w", err)
	}
	return cinemas, nil
}

func (s *CatalogService) ListMoviesByCinema(ctx context.Context, cinemaID int64) ([]domain.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	movies, err := s.repository.ListMoviesByCinema(ctx, cinemaID)
	if err != nil {
		return nil, fmt.Errorf("list movies by cinema: %w", err)
	}
	return movies, nil
}

func (s *CatalogService) GetMovieDetail(ctx context.Context, movieID int64) (*domain.MovieDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	movie, err := s.repository.GetMovieDetail(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("get movie detail: %w", err)
	}
	return movie, nil
}

func (s *CatalogService) ListShowtimesByCinemaAndMovie(
	ctx context.Context,
	cinemaID, movieID int64,
) ([]domain.Showtime, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	showtimes, err := s.repository.ListShowtimesByCinemaAndMovie(ctx, cinemaID, movieID)
	if err != nil {
		return nil, fmt.Errorf("list showtimes by cinema and movie: %w", err)
	}
	return showtimes, nil
}

func (s *CatalogService) GetSeatMap(ctx context.Context, showtimeID int64) (*domain.SeatMap, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	seatMap, err := s.repository.GetSeatMap(ctx, showtimeID)
	if err != nil {
		return nil, fmt.Errorf("get seat map: %w", err)
	}
	return seatMap, nil
}
