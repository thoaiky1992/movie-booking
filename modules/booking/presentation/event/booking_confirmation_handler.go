package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	bookingapplication "movie-booking/modules/booking/application"
)

// bookingConfirmationTaskPayload is the wire format this handler decodes. Its
// JSON field names must match the encoder in infrastructure/producer.
type bookingConfirmationTaskPayload struct {
	BookingID      int64     `json:"booking_id"`
	ShowtimeID     int64     `json:"showtime_id"`
	SeatIDs        []int64   `json:"seat_ids"`
	TotalAmount    string    `json:"total_amount"`
	ConfirmedAt    time.Time `json:"confirmed_at"`
	RecipientEmail string    `json:"recipient_email"`
}

type BookingConfirmationHandler struct {
	useCase *bookingapplication.SendBookingConfirmationCommand
}

func NewBookingConfirmationHandler(useCase *bookingapplication.SendBookingConfirmationCommand) *BookingConfirmationHandler {
	return &BookingConfirmationHandler{useCase: useCase}
}

func (h *BookingConfirmationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload bookingConfirmationTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode booking confirmation email task: %w: %w", err, asynq.SkipRetry)
	}
	email := bookingapplication.BookingConfirmationEmail{
		BookingID:      payload.BookingID,
		ShowtimeID:     payload.ShowtimeID,
		SeatIDs:        payload.SeatIDs,
		TotalAmount:    payload.TotalAmount,
		ConfirmedAt:    payload.ConfirmedAt,
		RecipientEmail: payload.RecipientEmail,
	}
	if err := h.useCase.Execute(ctx, email); err != nil {
		if errors.Is(err, bookingapplication.ErrInvalidBookingConfirmation) {
			return fmt.Errorf("invalid booking confirmation email payload: %w", asynq.SkipRetry)
		}
		return err
	}
	return nil
}
