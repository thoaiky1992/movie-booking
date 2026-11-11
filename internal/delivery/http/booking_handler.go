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
