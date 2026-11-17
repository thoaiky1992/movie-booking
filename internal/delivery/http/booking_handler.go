package http

import (
	"context"
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
	if err != nil {
		respondBookingError(c, err)
		return
	}

	RespondSuccess(c, nethttp.StatusOK, confirmed)
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
	if err != nil {
		respondBookingError(c, err)
		return
	}

	RespondSuccess(c, nethttp.StatusOK, cancelled)
}

type BookingHandler struct {
	useCase BookingUseCase
}

func NewBookingHandler(useCase BookingUseCase) *BookingHandler {
	return &BookingHandler{useCase: useCase}
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
	if err != nil {
		respondBookingError(c, err)
		return
	}

	RespondSuccess(c, nethttp.StatusCreated, hold)
}
