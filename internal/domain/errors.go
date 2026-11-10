package domain

import "errors"

// Sentinel errors. Adapters map these to transport codes; usecases wrap them
// with context via fmt.Errorf("...: %w", err).
var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrInvalidInput      = errors.New("invalid input")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidState      = errors.New("invalid state for this operation")
	ErrSeatUnavailable   = errors.New("seat unavailable")
	ErrHoldExpired       = errors.New("seat hold expired")
	ErrNotConfirmable    = errors.New("booking can no longer be confirmed")
	ErrShowtimeStarted   = errors.New("showtime has already started")
	ErrPaymentInProgress = errors.New("a payment is already in progress or completed")
	ErrPaymentDeclined   = errors.New("payment declined")
)
