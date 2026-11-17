package interfaces

import (
	"context"
	"time"

	"movie-booking/internal/domain"
)

// BookingConfirmationEmail is the content needed to send a booking confirmation.
// Queue names, task types and wire encoding are infrastructure concerns and
// live in the adapters.
type BookingConfirmationEmail struct {
	BookingID      int64
	ShowtimeID     int64
	SeatIDs        []int64
	TotalAmount    string
	ConfirmedAt    time.Time
	RecipientEmail string
}

type BookingConfirmationEnqueuer interface {
	EnqueueBookingConfirmation(context.Context, *domain.ConfirmedBooking) error
}

type Mailer interface {
	SendBookingConfirmation(context.Context, BookingConfirmationEmail) error
}
