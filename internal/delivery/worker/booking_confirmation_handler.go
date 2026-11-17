package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"movie-booking/internal/notification"
)

type BookingConfirmationHandler struct {
	mailer notification.Mailer
}

func NewBookingConfirmationHandler(mailer notification.Mailer) *BookingConfirmationHandler {
	return &BookingConfirmationHandler{mailer: mailer}
}

func (h *BookingConfirmationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload notification.BookingConfirmationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode booking confirmation email task: %w: %w", err, asynq.SkipRetry)
	}
	if payload.BookingID <= 0 || payload.RecipientEmail == "" {
		return fmt.Errorf("invalid booking confirmation email payload: %w", asynq.SkipRetry)
	}
	if err := h.mailer.SendBookingConfirmation(ctx, payload); err != nil {
		if errors.Is(err, asynq.SkipRetry) {
			return err
		}
		return fmt.Errorf("send booking confirmation email: %w", err)
	}
	return nil
}
