package http

import (
	"context"
	"errors"
	nethttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"movie-booking/internal/domain"
)

type CatalogUseCase interface {
	ListCinemas(ctx context.Context) ([]domain.Cinema, error)
	ListMoviesByCinema(ctx context.Context, cinemaID int64) ([]domain.Movie, error)
	GetMovieDetail(ctx context.Context, movieID int64) (*domain.MovieDetail, error)
	ListShowtimesByCinemaAndMovie(ctx context.Context, cinemaID, movieID int64) ([]domain.Showtime, error)
	GetSeatMap(ctx context.Context, showtimeID int64) (*domain.SeatMap, error)
}

type CatalogHandler struct {
	useCase CatalogUseCase
}

func NewCatalogHandler(useCase CatalogUseCase) *CatalogHandler {
	return &CatalogHandler{useCase: useCase}
}

func (h *CatalogHandler) ListCinemas(c *gin.Context) {
	cinemas, err := h.useCase.ListCinemas(c.Request.Context())
	if err != nil {
		respondInternalError(c, err)
		return
	}

	RespondSuccess(c, nethttp.StatusOK, cinemas)
}

func (h *CatalogHandler) ListMoviesByCinema(c *gin.Context) {
	cinemaID, ok := parsePositiveID(c, "cinemaID", "INVALID_CINEMA_ID", "cinema")
	if !ok {
		return
	}

	movies, err := h.useCase.ListMoviesByCinema(c.Request.Context(), cinemaID)
	if err != nil {
		respondInternalError(c, err)
		return
	}

	RespondSuccess(c, nethttp.StatusOK, movies)
}

func (h *CatalogHandler) GetMovieDetail(c *gin.Context) {
	movieID, ok := parsePositiveID(c, "movieID", "INVALID_MOVIE_ID", "movie")
	if !ok {
		return
	}

	movie, err := h.useCase.GetMovieDetail(c.Request.Context(), movieID)
	if errors.Is(err, domain.ErrMovieNotFound) {
		RespondError(c, nethttp.StatusNotFound, "MOVIE_NOT_FOUND", domain.ErrMovieNotFound.Error())
		return
	}
	if err != nil {
		respondInternalError(c, err)
		return
	}

	RespondSuccess(c, nethttp.StatusOK, movie)
}

func (h *CatalogHandler) ListShowtimesByCinemaAndMovie(c *gin.Context) {
	cinemaID, ok := parsePositiveID(c, "cinemaID", "INVALID_CINEMA_ID", "cinema")
	if !ok {
		return
	}
	movieID, ok := parsePositiveID(c, "movieID", "INVALID_MOVIE_ID", "movie")
	if !ok {
		return
	}

	showtimes, err := h.useCase.ListShowtimesByCinemaAndMovie(c.Request.Context(), cinemaID, movieID)
	if err != nil {
		respondInternalError(c, err)
		return
	}

	RespondSuccess(c, nethttp.StatusOK, showtimes)
}

func (h *CatalogHandler) GetSeatMap(c *gin.Context) {
	showtimeID, ok := parsePositiveID(c, "showtimeID", "INVALID_SHOWTIME_ID", "showtime")
	if !ok {
		return
	}

	seatMap, err := h.useCase.GetSeatMap(c.Request.Context(), showtimeID)
	if errors.Is(err, domain.ErrShowtimeNotFound) {
		RespondError(c, nethttp.StatusNotFound, "SHOWTIME_NOT_FOUND", domain.ErrShowtimeNotFound.Error())
		return
	}
	if err != nil {
		respondInternalError(c, err)
		return
	}

	RespondSuccess(c, nethttp.StatusOK, seatMap)
}

func parsePositiveID(c *gin.Context, parameter, code, resource string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(parameter), 10, 64)
	if err != nil || id <= 0 {
		RespondError(
			c,
			nethttp.StatusBadRequest,
			code,
			resource+" id must be a positive integer",
		)
		return 0, false
	}
	return id, true
}

func respondInternalError(c *gin.Context, err error) {
	_ = c.Error(err)
	RespondError(c, nethttp.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred")
}
