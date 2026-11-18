package http_api

import (
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	"movie-booking/modules/catalog/application"
	httpdelivery "movie-booking/shared/httpapi"
)

type Handler struct {
	listCinemas        *application.ListCinemasQuery
	listMoviesByCinema *application.ListMoviesByCinemaQuery
	getMovieDetail     *application.GetMovieDetailQuery
	listShowtimes      *application.ListShowtimesQuery
	getSeatMap         *application.GetSeatMapQuery
}

func NewHandler(
	listCinemas *application.ListCinemasQuery,
	listMoviesByCinema *application.ListMoviesByCinemaQuery,
	getMovieDetail *application.GetMovieDetailQuery,
	listShowtimes *application.ListShowtimesQuery,
	getSeatMap *application.GetSeatMapQuery,
) *Handler {
	return &Handler{
		listCinemas: listCinemas, listMoviesByCinema: listMoviesByCinema,
		getMovieDetail: getMovieDetail, listShowtimes: listShowtimes,
		getSeatMap: getSeatMap,
	}
}

func (h *Handler) ListCinemas(c *gin.Context) {
	cinemas, err := h.listCinemas.Execute(c.Request.Context())
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

	movies, err := h.listMoviesByCinema.Execute(c.Request.Context(), cinemaID)
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

	movie, err := h.getMovieDetail.Execute(c.Request.Context(), movieID)
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

	showtimes, err := h.listShowtimes.Execute(c.Request.Context(), cinemaID, movieID)
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

	seatMap, err := h.getSeatMap.Execute(c.Request.Context(), showtimeID)
	if err != nil {
		respondCatalogError(c, err)
		return
	}

	httpdelivery.RespondSuccess(c, nethttp.StatusOK, newSeatMapResponse(seatMap))
}
