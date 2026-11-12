package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/hibiken/asynq"

	"movie-booking/internal/domain"
)

const (
	BookingConfirmationTaskType = "email:booking_confirmation"
	EmailQueue                  = "email"
)

type BookingConfirmationPayload struct {
	BookingID      int64     `json:"booking_id"`
	ShowtimeID     int64     `json:"showtime_id"`
	SeatIDs        []int64   `json:"seat_ids"`
	TotalAmount    string    `json:"total_amount"`
	ConfirmedAt    time.Time `json:"confirmed_at"`
	RecipientEmail string    `json:"recipient_email"`
}

type AsynqEnqueuer struct {
	client *asynq.Client
}

func NewAsynqEnqueuer(redisOption asynq.RedisConnOpt) *AsynqEnqueuer {
	return &AsynqEnqueuer{client: asynq.NewClient(redisOption)}
}

func (e *AsynqEnqueuer) EnqueueBookingConfirmation(
	ctx context.Context,
	booking *domain.ConfirmedBooking,
) error {
	payload, err := json.Marshal(BookingConfirmationPayload{
		BookingID:      booking.BookingID,
		ShowtimeID:     booking.ShowtimeID,
		SeatIDs:        booking.SeatIDs,
		TotalAmount:    booking.TotalAmount,
		ConfirmedAt:    booking.ConfirmedAt,
		RecipientEmail: booking.RecipientEmail,
	})
	if err != nil {
		return fmt.Errorf("marshal booking confirmation email task: %w", err)
	}

	task := asynq.NewTask(BookingConfirmationTaskType, payload)
	_, err = e.client.EnqueueContext(
		ctx,
		task,
		asynq.Queue(EmailQueue),
		asynq.TaskID("booking-confirmation-email:"+strconv.FormatInt(booking.BookingID, 10)),
		asynq.MaxRetry(5),
		asynq.Timeout(30*time.Second),
		asynq.Retention(24*time.Hour),
	)
	if err != nil {
		// A retry of the idempotent confirm endpoint may enqueue the same task.
		// The existing task is already pending, active, or retained as completed.
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			return nil
		}
		return fmt.Errorf("enqueue booking confirmation email: %w", err)
	}
	return nil
}

func (e *AsynqEnqueuer) Close() error {
	return e.client.Close()
}

type Mailer interface {
	SendBookingConfirmation(context.Context, BookingConfirmationPayload) error
}

// MockMailer intentionally does not contact an external email provider. It
// records the successful send in the worker log so the background flow can be
// exercised end-to-end in local development.
type MockMailer struct {
	logger *slog.Logger
}

func NewMockMailer(logger *slog.Logger) *MockMailer {
	return &MockMailer{logger: logger}
}

func (m *MockMailer) SendBookingConfirmation(
	ctx context.Context,
	payload BookingConfirmationPayload,
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

type BookingConfirmationHandler struct {
	mailer Mailer
}

func NewBookingConfirmationHandler(mailer Mailer) *BookingConfirmationHandler {
	return &BookingConfirmationHandler{mailer: mailer}
}

func (h *BookingConfirmationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload BookingConfirmationPayload
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
