package domain

import (
	"errors"
	"time"
)

var (
	ErrSeatsUnavailable      = errors.New("one or more seats are unavailable")
	ErrInvalidSeatHold       = errors.New("invalid seat hold request")
	ErrShowtimeNotFound      = errors.New("showtime not found")
	ErrShowtimeStarted       = errors.New("showtime has already started")
	ErrInvalidBookingID      = errors.New("invalid booking ID")
	ErrBookingNotFound       = errors.New("booking not found")
	ErrBookingNotCancellable = errors.New("booking cannot be cancelled")
	ErrBookingNotConfirmable = errors.New("booking cannot be confirmed")
)

const MaxSeatsPerHold = 10

type SeatHold struct {
	BookingID   int64
	ShowtimeID  int64
	SeatIDs     []int64
	HoldToken   string
	Status      string
	TotalAmount string
	ExpiresAt   time.Time
}

type ReleasedSeat struct {
	ShowtimeID int64
	SeatID     int64
	HoldToken  string
}

type CancelledBooking struct {
	BookingID  int64
	ShowtimeID int64
	SeatIDs    []int64
	Status     string
	HoldToken  string
}

type ConfirmedBooking struct {
	BookingID      int64
	ShowtimeID     int64
	SeatIDs        []int64
	Status         string
	TotalAmount    string
	ConfirmedAt    time.Time
	RecipientEmail string
	HoldToken      string
}
