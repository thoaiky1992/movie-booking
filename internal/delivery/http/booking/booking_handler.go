package booking

import (
	"context"
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
	"movie-booking/internal/domain"
)

type UseCase interface {
	HoldSeats(
		ctx context.Context,
		userID int64,
		showtimeID int64,
		seatIDs []int64,
	) (*domain.SeatHold, error)
	ConfirmBooking(ctx context.Context, userID, bookingID int64) (*domain.ConfirmedBooking, error)
	CancelBooking(ctx context.Context, userID, bookingID int64) (*domain.CancelledBooking, error)
}

type Handler struct {
	useCase UseCase
}

func NewHandler(useCase UseCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) ConfirmBooking(c *gin.Context) {
	userID, ok := httpdelivery.UserIDFromContext(c.Request.Context())
	if !ok {
		httpdelivery.RespondError(c, nethttp.StatusUnauthorized, "UNAUTHORIZED", "user identity is missing")
		return
	}
	bookingID, ok := httpdelivery.ParsePositiveID(c, "bookingID", "INVALID_BOOKING_ID", "booking")
	if !ok {
		return
	}

	confirmed, err := h.useCase.ConfirmBooking(c.Request.Context(), userID, bookingID)
	if err != nil {
		respondBookingError(c, err)
		return
	}

	httpdelivery.RespondSuccess(c, nethttp.StatusOK, newConfirmedBookingResponse(confirmed))
}

func (h *Handler) CancelBooking(c *gin.Context) {
	userID, ok := httpdelivery.UserIDFromContext(c.Request.Context())
	if !ok {
		httpdelivery.RespondError(c, nethttp.StatusUnauthorized, "UNAUTHORIZED", "user identity is missing")
		return
	}
	bookingID, ok := httpdelivery.ParsePositiveID(c, "bookingID", "INVALID_BOOKING_ID", "booking")
	if !ok {
		return
	}

	cancelled, err := h.useCase.CancelBooking(c.Request.Context(), userID, bookingID)
	if err != nil {
		respondBookingError(c, err)
		return
	}

	httpdelivery.RespondSuccess(c, nethttp.StatusOK, newCancelledBookingResponse(cancelled))
}

func (h *Handler) HoldSeats(c *gin.Context) {
	userID, ok := httpdelivery.UserIDFromContext(c.Request.Context())
	if !ok {
		httpdelivery.RespondError(c, nethttp.StatusUnauthorized, "UNAUTHORIZED", "user identity is missing")
		return
	}

	showtimeID, ok := httpdelivery.ParsePositiveID(c, "showtimeID", "INVALID_SHOWTIME_ID", "showtime")
	if !ok {
		return
	}

	var request holdSeatsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpdelivery.RespondError(
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

	httpdelivery.RespondSuccess(c, nethttp.StatusCreated, newSeatHoldResponse(hold))
}
