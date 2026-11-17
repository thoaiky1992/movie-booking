package booking

import (
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
	"movie-booking/internal/domain"
)

// respondBookingError translates domain errors into the HTTP API contract.
// Domain and use-case layers remain independent of HTTP status codes.
func respondBookingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidSeatHold):
		httpdelivery.RespondError(c, nethttp.StatusBadRequest, "INVALID_HOLD_REQUEST", domain.ErrInvalidSeatHold.Error())
	case errors.Is(err, domain.ErrShowtimeNotFound):
		httpdelivery.RespondError(c, nethttp.StatusNotFound, "SHOWTIME_NOT_FOUND", domain.ErrShowtimeNotFound.Error())
	case errors.Is(err, domain.ErrShowtimeStarted):
		httpdelivery.RespondError(c, nethttp.StatusConflict, "SHOWTIME_STARTED", domain.ErrShowtimeStarted.Error())
	case errors.Is(err, domain.ErrSeatsUnavailable):
		httpdelivery.RespondError(c, nethttp.StatusConflict, "SEATS_UNAVAILABLE", domain.ErrSeatsUnavailable.Error())
	case errors.Is(err, domain.ErrInvalidBookingID):
		httpdelivery.RespondError(c, nethttp.StatusBadRequest, "INVALID_BOOKING_ID", domain.ErrInvalidBookingID.Error())
	case errors.Is(err, domain.ErrBookingNotFound):
		httpdelivery.RespondError(c, nethttp.StatusNotFound, "BOOKING_NOT_FOUND", domain.ErrBookingNotFound.Error())
	case errors.Is(err, domain.ErrBookingNotConfirmable):
		httpdelivery.RespondError(c, nethttp.StatusConflict, "BOOKING_NOT_CONFIRMABLE", domain.ErrBookingNotConfirmable.Error())
	case errors.Is(err, domain.ErrBookingNotCancellable):
		httpdelivery.RespondError(c, nethttp.StatusConflict, "BOOKING_NOT_CANCELLABLE", domain.ErrBookingNotCancellable.Error())
	default:
		httpdelivery.RespondInternalError(c, err)
	}
}
