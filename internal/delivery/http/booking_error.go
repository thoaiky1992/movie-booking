package http

import (
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	"movie-booking/internal/domain"
)

// respondBookingError translates domain errors into the HTTP API contract.
// Domain and use-case layers remain independent of HTTP status codes.
func respondBookingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidSeatHold):
		RespondError(c, nethttp.StatusBadRequest, "INVALID_HOLD_REQUEST", domain.ErrInvalidSeatHold.Error())
	case errors.Is(err, domain.ErrShowtimeNotFound):
		RespondError(c, nethttp.StatusNotFound, "SHOWTIME_NOT_FOUND", domain.ErrShowtimeNotFound.Error())
	case errors.Is(err, domain.ErrShowtimeStarted):
		RespondError(c, nethttp.StatusConflict, "SHOWTIME_STARTED", domain.ErrShowtimeStarted.Error())
	case errors.Is(err, domain.ErrSeatsUnavailable):
		RespondError(c, nethttp.StatusConflict, "SEATS_UNAVAILABLE", domain.ErrSeatsUnavailable.Error())
	case errors.Is(err, domain.ErrInvalidBookingID):
		RespondError(c, nethttp.StatusBadRequest, "INVALID_BOOKING_ID", domain.ErrInvalidBookingID.Error())
	case errors.Is(err, domain.ErrBookingNotFound):
		RespondError(c, nethttp.StatusNotFound, "BOOKING_NOT_FOUND", domain.ErrBookingNotFound.Error())
	case errors.Is(err, domain.ErrBookingNotConfirmable):
		RespondError(c, nethttp.StatusConflict, "BOOKING_NOT_CONFIRMABLE", domain.ErrBookingNotConfirmable.Error())
	case errors.Is(err, domain.ErrBookingNotCancellable):
		RespondError(c, nethttp.StatusConflict, "BOOKING_NOT_CANCELLABLE", domain.ErrBookingNotCancellable.Error())
	default:
		respondInternalError(c, err)
	}
}
