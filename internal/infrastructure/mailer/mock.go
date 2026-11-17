package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"movie-booking/internal/notification"
)

type Mock struct {
	logger *slog.Logger
}

func NewMock(logger *slog.Logger) *Mock {
	return &Mock{logger: logger}
}

func (m *Mock) SendBookingConfirmation(
	ctx context.Context,
	payload notification.BookingConfirmationPayload,
) error {
	delay := time.Second + time.Duration(rand.IntN(1001))*time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("mock email send cancelled: %w", ctx.Err())
	case <-timer.C:
	}

	m.logger.InfoContext(
		ctx,
		"mock_booking_confirmation_email_sent",
		"booking_id", payload.BookingID,
		"recipient", payload.RecipientEmail,
		"duration_ms", delay.Milliseconds(),
		"message", "booking confirmation email sent successfully",
	)
	return nil
}
