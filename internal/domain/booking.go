package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSeatsUnavailable      = errors.New("one or more seats are unavailable")
	ErrInvalidSeatHold       = errors.New("invalid seat hold request")
	ErrShowtimeStarted       = errors.New("showtime has already started")
	ErrInvalidBookingID      = errors.New("invalid booking ID")
	ErrBookingNotFound       = errors.New("booking not found")
	ErrBookingNotCancellable = errors.New("booking cannot be cancelled")
)

const MaxSeatsPerHold = 10

type SeatHold struct {
	BookingID   int64     `json:"booking_id"`
	ShowtimeID  int64     `json:"showtime_id"`
	SeatIDs     []int64   `json:"seat_ids"`
	HoldToken   string    `json:"hold_token"`
	Status      string    `json:"status"`
	TotalAmount string    `json:"total_amount"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type ReleasedSeat struct {
	ShowtimeID int64
	SeatID     int64
	HoldToken  string
}

type CancelledBooking struct {
	BookingID  int64   `json:"booking_id"`
	ShowtimeID int64   `json:"showtime_id"`
	SeatIDs    []int64 `json:"seat_ids"`
	Status     string  `json:"status"`
	HoldToken  string  `json:"-"`
}

// BookingRepository owns the durable booking invariant. Implementations must
// claim all requested seats in one transaction or claim none of them.
type BookingRepository interface {
	HoldSeats(
		ctx context.Context,
		userID int64,
		showtimeID int64,
		seatIDs []int64,
		holdToken string,
		ttl time.Duration,
	) (*SeatHold, error)
	CancelBooking(ctx context.Context, userID, bookingID int64) (*CancelledBooking, error)
	ExpirePendingHolds(ctx context.Context, batchSize int) ([]ReleasedSeat, error)
}
