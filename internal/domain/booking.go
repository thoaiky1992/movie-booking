package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSeatsUnavailable = errors.New("one or more seats are unavailable")
	ErrInvalidSeatHold  = errors.New("invalid seat hold request")
	ErrShowtimeStarted  = errors.New("showtime has already started")
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
	ExpirePendingHolds(ctx context.Context, batchSize int) ([]ReleasedSeat, error)
}
