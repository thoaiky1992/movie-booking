package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	"movie-booking/internal/domain"
)

type BookingUseCase interface {
	HoldSeats(
		ctx context.Context,
		userID int64,
		showtimeID int64,
		seatIDs []int64,
	) (*domain.SeatHold, error)
	ConfirmBooking(ctx context.Context, userID, bookingID int64) (*domain.ConfirmedBooking, error)
	CancelBooking(ctx context.Context, userID, bookingID int64) (*domain.CancelledBooking, error)
}

func (h *BookingHandler) ConfirmBooking(c *gin.Context) {
	userID, ok := UserIDFromContext(c.Request.Context())
	if !ok {
		RespondError(c, nethttp.StatusUnauthorized, "UNAUTHORIZED", "user identity is missing")
		return
	}
	bookingID, ok := parsePositiveID(c, "bookingID", "INVALID_BOOKING_ID", "booking")
	if !ok {
		return
	}

	confirmed, err := h.useCase.ConfirmBooking(c.Request.Context(), userID, bookingID)
	switch {
	case err == nil:
		RespondSuccess(c, nethttp.StatusOK, confirmed)
	case errors.Is(err, domain.ErrInvalidBookingID):
		RespondError(c, nethttp.StatusBadRequest, "INVALID_BOOKING_ID", err.Error())
	case errors.Is(err, domain.ErrBookingNotFound):
		RespondError(c, nethttp.StatusNotFound, "BOOKING_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrBookingNotConfirmable):
		RespondError(c, nethttp.StatusConflict, "BOOKING_NOT_CONFIRMABLE", err.Error())
	default:
		respondInternalError(c, err)
	}
}

func (h *BookingHandler) CancelBooking(c *gin.Context) {
	userID, ok := UserIDFromContext(c.Request.Context())
	if !ok {
		RespondError(c, nethttp.StatusUnauthorized, "UNAUTHORIZED", "user identity is missing")
		return
	}
	bookingID, ok := parsePositiveID(c, "bookingID", "INVALID_BOOKING_ID", "booking")
	if !ok {
		return
	}

	cancelled, err := h.useCase.CancelBooking(c.Request.Context(), userID, bookingID)
	switch {
	case err == nil:
		RespondSuccess(c, nethttp.StatusOK, cancelled)
	case errors.Is(err, domain.ErrInvalidBookingID):
		RespondError(c, nethttp.StatusBadRequest, "INVALID_BOOKING_ID", err.Error())
	case errors.Is(err, domain.ErrBookingNotFound):
		RespondError(c, nethttp.StatusNotFound, "BOOKING_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrBookingNotCancellable):
		RespondError(c, nethttp.StatusConflict, "BOOKING_NOT_CANCELLABLE", err.Error())
	default:
		respondInternalError(c, err)
	}
}

type BookingHandler struct {
	useCase BookingUseCase
}

func NewBookingHandler(useCase BookingUseCase) *BookingHandler {
	return &BookingHandler{useCase: useCase}
}

type holdSeatsRequest struct {
	SeatIDs []int64 `json:"seat_ids" binding:"required,min=1,max=10,dive,gt=0"`
}

func (h *BookingHandler) HoldSeats(c *gin.Context) {
	userID, ok := UserIDFromContext(c.Request.Context())
	if !ok {
		RespondError(c, nethttp.StatusUnauthorized, "UNAUTHORIZED", "user identity is missing")
		return
	}

	showtimeID, ok := parsePositiveID(c, "showtimeID", "INVALID_SHOWTIME_ID", "showtime")
	if !ok {
		return
	}

	var request holdSeatsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		RespondError(
			c,
			nethttp.StatusBadRequest,
			"INVALID_HOLD_REQUEST",
			"1 to 10 unique positive seat_ids are required",
		)
		return
	}

	hold, err := h.useCase.HoldSeats(
		c.Request.Context(),
		userID,
		showtimeID,
		request.SeatIDs,
	)
	switch {
	case err == nil:
		RespondSuccess(c, nethttp.StatusCreated, hold)
	case errors.Is(err, domain.ErrInvalidSeatHold):
		RespondError(c, nethttp.StatusBadRequest, "INVALID_HOLD_REQUEST", err.Error())
	case errors.Is(err, domain.ErrShowtimeNotFound):
		RespondError(c, nethttp.StatusNotFound, "SHOWTIME_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrShowtimeStarted):
		RespondError(c, nethttp.StatusConflict, "SHOWTIME_STARTED", err.Error())
	case errors.Is(err, domain.ErrSeatsUnavailable):
		RespondError(c, nethttp.StatusConflict, "SEATS_UNAVAILABLE", err.Error())
	default:
		respondInternalError(c, err)
	}
}
