package notification

import (
	"context"
	"time"

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

type BookingConfirmationEnqueuer interface {
	EnqueueBookingConfirmation(context.Context, *domain.ConfirmedBooking) error
}

type Mailer interface {
	SendBookingConfirmation(context.Context, BookingConfirmationPayload) error
}
