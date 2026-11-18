package application

import (
	"context"
	"time"

	"movie-booking/modules/booking/domain"
)

// Repository ports are owned by the application layer. Infrastructure
// adapters implement these contracts without leaking storage details inward.
type BookingRepository interface {
	HoldSeats(context.Context, domain.UserID, domain.ShowtimeID, []domain.SeatID, string, time.Duration) (*domain.SeatHold, error)
	ConfirmBooking(context.Context, domain.UserID, domain.BookingID) (*domain.ConfirmedBooking, error)
	CancelBooking(context.Context, domain.UserID, domain.BookingID) (*domain.CancelledBooking, error)
}

type PendingHoldRepository interface {
	ExpirePendingHolds(context.Context, int) ([]domain.ReleasedSeat, error)
}

type SeatLocker interface {
	Acquire(context.Context, int64, []int64, string, time.Duration) (bool, error)
	Release(context.Context, int64, []int64, string) error
}

type ExpiredSeatLockReleaser interface {
	Release(context.Context, int64, []int64, string) error
}

type BookingConfirmationEnqueuer interface {
	EnqueueBookingConfirmation(context.Context, *domain.ConfirmedBooking) error
}

type BookingConfirmationEmail struct {
	BookingID      int64
	ShowtimeID     int64
	SeatIDs        []int64
	TotalAmount    string
	ConfirmedAt    time.Time
	RecipientEmail string
}

type Mailer interface {
	SendBookingConfirmation(context.Context, BookingConfirmationEmail) error
}

type Logger interface {
	WarnContext(context.Context, string, ...any)
	ErrorContext(context.Context, string, ...any)
}
