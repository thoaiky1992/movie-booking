package asynq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	asynqclient "github.com/hibiken/asynq"

	"movie-booking/internal/domain"
	"movie-booking/internal/usecase/interfaces"
)

const (
	BookingConfirmationTaskType = "email:booking_confirmation"
	EmailQueue                  = "email"
)

var _ interfaces.BookingConfirmationEnqueuer = (*BookingConfirmationEnqueuer)(nil)

// bookingConfirmationTask is the wire format of the email task. Its JSON field
// names are part of the queue contract and must match the decoder in
// delivery/worker.
type bookingConfirmationTask struct {
	BookingID      int64     `json:"booking_id"`
	ShowtimeID     int64     `json:"showtime_id"`
	SeatIDs        []int64   `json:"seat_ids"`
	TotalAmount    string    `json:"total_amount"`
	ConfirmedAt    time.Time `json:"confirmed_at"`
	RecipientEmail string    `json:"recipient_email"`
}

type BookingConfirmationEnqueuer struct {
	client *asynqclient.Client
}

func NewBookingConfirmationEnqueuer(redisURL string) (*BookingConfirmationEnqueuer, error) {
	redisOption, err := asynqclient.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse Asynq Redis configuration: %w", err)
	}
	return &BookingConfirmationEnqueuer{client: asynqclient.NewClient(redisOption)}, nil
}

func (e *BookingConfirmationEnqueuer) EnqueueBookingConfirmation(
	ctx context.Context,
	booking *domain.ConfirmedBooking,
) error {
	payload, err := json.Marshal(bookingConfirmationTask{
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

	task := asynqclient.NewTask(BookingConfirmationTaskType, payload)
	_, err = e.client.EnqueueContext(
		ctx,
		task,
		asynqclient.Queue(EmailQueue),
		asynqclient.TaskID("booking-confirmation-email:"+strconv.FormatInt(booking.BookingID, 10)),
		asynqclient.MaxRetry(5),
		asynqclient.Timeout(30*time.Second),
		asynqclient.Retention(24*time.Hour),
	)
	if errors.Is(err, asynqclient.ErrTaskIDConflict) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("enqueue booking confirmation email: %w", err)
	}
	return nil
}

func (e *BookingConfirmationEnqueuer) Close() error {
	return e.client.Close()
}
