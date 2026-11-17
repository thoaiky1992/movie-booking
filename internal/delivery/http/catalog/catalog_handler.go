package catalog

import (
	"context"
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
	"movie-booking/internal/domain"
)

type UseCase interface {
	ListCinemas(ctx context.Context) ([]domain.Cinema, error)
	ListMoviesByCinema(ctx context.Context, cinemaID int64) ([]domain.Movie, error)
	GetMovieDetail(ctx context.Context, movieID int64) (*domain.MovieDetail, error)
	ListShowtimesByCinemaAndMovie(ctx context.Context, cinemaID, movieID int64) ([]domain.Showtime, error)
	GetSeatMap(ctx context.Context, showtimeID int64) (*domain.SeatMap, error)
}

type Handler struct {
	useCase UseCase
}

func NewHandler(useCase UseCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) ListCinemas(c *gin.Context) {
	cinemas, err := h.useCase.ListCinemas(c.Request.Context())
	if err != nil {
		respondCatalogError(c, err)
		return
	}

	httpdelivery.RespondSuccess(c, nethttp.StatusOK, newCinemaResponses(cinemas))
}

func (h *Handler) ListMoviesByCinema(c *gin.Context) {
	cinemaID, ok := httpdelivery.ParsePositiveID(c, "cinemaID", "INVALID_CINEMA_ID", "cinema")
	if !ok {
		return
	}

	movies, err := h.useCase.ListMoviesByCinema(c.Request.Context(), cinemaID)
	if err != nil {
		respondCatalogError(c, err)
		return
	}

	httpdelivery.RespondSuccess(c, nethttp.StatusOK, newMovieResponses(movies))
}

func (h *Handler) GetMovieDetail(c *gin.Context) {
	movieID, ok := httpdelivery.ParsePositiveID(c, "movieID", "INVALID_MOVIE_ID", "movie")
	if !ok {
		return
	}

	movie, err := h.useCase.GetMovieDetail(c.Request.Context(), movieID)
	if err != nil {
		respondCatalogError(c, err)
		return
	}

	httpdelivery.RespondSuccess(c, nethttp.StatusOK, newMovieDetailResponse(movie))
}

func (h *Handler) ListShowtimesByCinemaAndMovie(c *gin.Context) {
	cinemaID, ok := httpdelivery.ParsePositiveID(c, "cinemaID", "INVALID_CINEMA_ID", "cinema")
	if !ok {
		return
	}
	movieID, ok := httpdelivery.ParsePositiveID(c, "movieID", "INVALID_MOVIE_ID", "movie")
	if !ok {
		return
	}

	showtimes, err := h.useCase.ListShowtimesByCinemaAndMovie(c.Request.Context(), cinemaID, movieID)
	if err != nil {
		respondCatalogError(c, err)
		return
	}

	httpdelivery.RespondSuccess(c, nethttp.StatusOK, newShowtimeResponses(showtimes))
}

func (h *Handler) GetSeatMap(c *gin.Context) {
	showtimeID, ok := httpdelivery.ParsePositiveID(c, "showtimeID", "INVALID_SHOWTIME_ID", "showtime")
	if !ok {
		return
	}

	seatMap, err := h.useCase.GetSeatMap(c.Request.Context(), showtimeID)
	if err != nil {
		respondCatalogError(c, err)
		return
	}

	httpdelivery.RespondSuccess(c, nethttp.StatusOK, newSeatMapResponse(seatMap))
}
