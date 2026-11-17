package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	bookingusecase "movie-booking/internal/usecase/booking"
	"movie-booking/internal/usecase/interfaces"
)

type BookingConfirmationUseCase interface {
	Send(ctx context.Context, email interfaces.BookingConfirmationEmail) error
}

// bookingConfirmationTaskPayload is the wire format this handler decodes. Its
// JSON field names must match the encoder in infrastructure/asynq.
type bookingConfirmationTaskPayload struct {
	BookingID      int64     `json:"booking_id"`
	ShowtimeID     int64     `json:"showtime_id"`
	SeatIDs        []int64   `json:"seat_ids"`
	TotalAmount    string    `json:"total_amount"`
	ConfirmedAt    time.Time `json:"confirmed_at"`
	RecipientEmail string    `json:"recipient_email"`
}

type BookingConfirmationHandler struct {
	useCase BookingConfirmationUseCase
}

func NewBookingConfirmationHandler(useCase BookingConfirmationUseCase) *BookingConfirmationHandler {
	return &BookingConfirmationHandler{useCase: useCase}
}

func (h *BookingConfirmationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload bookingConfirmationTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode booking confirmation email task: %w: %w", err, asynq.SkipRetry)
	}
	email := interfaces.BookingConfirmationEmail{
		BookingID:      payload.BookingID,
		ShowtimeID:     payload.ShowtimeID,
		SeatIDs:        payload.SeatIDs,
		TotalAmount:    payload.TotalAmount,
		ConfirmedAt:    payload.ConfirmedAt,
		RecipientEmail: payload.RecipientEmail,
	}
	if err := h.useCase.Send(ctx, email); err != nil {
		if errors.Is(err, bookingusecase.ErrInvalidBookingConfirmation) {
			return fmt.Errorf("invalid booking confirmation email payload: %w", asynq.SkipRetry)
		}
		return err
	}
	return nil
}
