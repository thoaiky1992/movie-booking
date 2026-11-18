package http_api

import (
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	"movie-booking/modules/booking/application"
	httpdelivery "movie-booking/shared/httpapi"
)

type Handler struct {
	holdSeats      *application.HoldSeatsCommand
	confirmBooking *application.ConfirmBookingCommand
	cancelBooking  *application.CancelBookingCommand
}

func NewHandler(
	holdSeats *application.HoldSeatsCommand,
	confirmBooking *application.ConfirmBookingCommand,
	cancelBooking *application.CancelBookingCommand,
) *Handler {
	return &Handler{
		holdSeats: holdSeats, confirmBooking: confirmBooking, cancelBooking: cancelBooking,
	}
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

	confirmed, err := h.confirmBooking.Execute(c.Request.Context(), userID, bookingID)
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

	cancelled, err := h.cancelBooking.Execute(c.Request.Context(), userID, bookingID)
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

	hold, err := h.holdSeats.Execute(
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
