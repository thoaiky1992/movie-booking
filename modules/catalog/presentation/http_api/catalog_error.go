package http_api

import (
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	"movie-booking/modules/catalog/domain"
	httpdelivery "movie-booking/shared/httpapi"
)

func respondCatalogError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidCinemaID):
		httpdelivery.RespondError(c, nethttp.StatusBadRequest, "INVALID_CINEMA_ID", domain.ErrInvalidCinemaID.Error())
	case errors.Is(err, domain.ErrInvalidMovieID):
		httpdelivery.RespondError(c, nethttp.StatusBadRequest, "INVALID_MOVIE_ID", domain.ErrInvalidMovieID.Error())
	case errors.Is(err, domain.ErrInvalidShowtimeID):
		httpdelivery.RespondError(c, nethttp.StatusBadRequest, "INVALID_SHOWTIME_ID", domain.ErrInvalidShowtimeID.Error())
	case errors.Is(err, domain.ErrMovieNotFound):
		httpdelivery.RespondError(c, nethttp.StatusNotFound, "MOVIE_NOT_FOUND", domain.ErrMovieNotFound.Error())
	case errors.Is(err, domain.ErrShowtimeNotFound):
		httpdelivery.RespondError(c, nethttp.StatusNotFound, "SHOWTIME_NOT_FOUND", domain.ErrShowtimeNotFound.Error())
	default:
		httpdelivery.RespondInternalError(c, err)
	}
}
