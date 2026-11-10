package domain

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type BookingStatus string

const (
	BookingHeld      BookingStatus = "HELD"      // seats locked, awaiting payment until ExpiresAt
	BookingConfirmed BookingStatus = "CONFIRMED" // paid, seats sold
	BookingCancelled BookingStatus = "CANCELLED" // cancelled by user (seats released)
	BookingExpired   BookingStatus = "EXPIRED"   // hold TTL elapsed before payment
)

// Refund policy: cancellation windows relative to showtime start.
const (
	FullRefundWindow = 24 * time.Hour
	HalfRefundWindow = 2 * time.Hour
)

type SeatLine struct {
	SeatID     uuid.UUID `json:"seat_id"`
	RowLabel   string    `json:"row_label"`
	SeatNumber int       `json:"seat_number"`
	SeatType   string    `json:"seat_type"`
	PriceCents int64     `json:"price_cents"`
}

type Booking struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ShowtimeID  uuid.UUID
	Status      BookingStatus
	TotalCents  int64
	TicketCode  string
	Seats       []SeatLine
	ExpiresAt   *time.Time
	ConfirmedAt *time.Time
	CancelledAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewHold builds a HELD booking. It enforces every rule that can be checked
// without I/O; the uniqueness of seats is enforced later by the database.
func NewHold(userID uuid.UUID, show Showtime, seats []Seat, now time.Time, ttl time.Duration, maxSeats int) (*Booking, error) {
	if len(seats) == 0 {
		return nil, fmt.Errorf("%w: at least one seat is required", ErrInvalidInput)
	}
	if len(seats) > maxSeats {
		return nil, fmt.Errorf("%w: at most %d seats per booking", ErrInvalidInput, maxSeats)
	}
	if !show.StartsAt.After(now) {
		return nil, ErrShowtimeStarted
	}

	seen := make(map[uuid.UUID]struct{}, len(seats))
	lines := make([]SeatLine, 0, len(seats))
	var total int64
	for _, s := range seats {
		if s.HallID != show.HallID {
			return nil, fmt.Errorf("%w: seat %s does not belong to this showtime's hall", ErrInvalidInput, s.ID)
		}
		if _, dup := seen[s.ID]; dup {
			return nil, fmt.Errorf("%w: duplicate seat %s", ErrInvalidInput, s.ID)
		}
		seen[s.ID] = struct{}{}

		price := show.BasePriceCents + s.PriceModifierCents
		total += price
		lines = append(lines, SeatLine{
			SeatID:     s.ID,
			RowLabel:   s.RowLabel,
			SeatNumber: s.SeatNumber,
			SeatType:   s.SeatType,
			PriceCents: price,
		})
	}

	code, err := NewTicketCode()
	if err != nil {
		return nil, err
	}
	expires := now.Add(ttl)
	return &Booking{
		ID:         uuid.New(),
		UserID:     userID,
		ShowtimeID: show.ID,
		Status:     BookingHeld,
		TotalCents: total,
		TicketCode: code,
		Seats:      lines,
		ExpiresAt:  &expires,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// SeatIDs returns the seats covered by the booking, in booking order.
func (b *Booking) SeatIDs() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(b.Seats))
	for _, s := range b.Seats {
		ids = append(ids, s.SeatID)
	}
	return ids
}

// IsPayable reports whether the booking can still accept a payment at now.
func (b *Booking) IsPayable(now time.Time) error {
	if b.Status != BookingHeld {
		return fmt.Errorf("%w: booking is %s", ErrInvalidState, b.Status)
	}
	if b.ExpiresAt == nil || !now.Before(*b.ExpiresAt) {
		return ErrHoldExpired
	}
	return nil
}

// CancellationRefund returns the refund owed when the booking is cancelled at now.
// HELD bookings release seats with no refund (nothing was paid).
func CancellationRefund(b Booking, startsAt, now time.Time) (int64, error) {
	switch b.Status {
	case BookingHeld:
		return 0, nil
	case BookingConfirmed:
		if !now.Before(startsAt) {
			return 0, ErrShowtimeStarted
		}
		remaining := startsAt.Sub(now)
		switch {
		case remaining >= FullRefundWindow:
			return b.TotalCents, nil
		case remaining >= HalfRefundWindow:
			return b.TotalCents / 2, nil
		default:
			return 0, nil
		}
	default:
		return 0, fmt.Errorf("%w: booking is %s", ErrInvalidState, b.Status)
	}
}

// NewTicketCode returns a 16-character, unguessable, human-friendly code.
func NewTicketCode() (string, error) {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate ticket code: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

type PaymentStatus string

const (
	PaymentPending       PaymentStatus = "PENDING"
	PaymentSucceeded     PaymentStatus = "SUCCEEDED"
	PaymentFailed        PaymentStatus = "FAILED"
	PaymentRefundPending PaymentStatus = "REFUND_PENDING"
	PaymentRefunded      PaymentStatus = "REFUNDED"
	PaymentNoRefund      PaymentStatus = "NO_REFUND"
)

type Payment struct {
	ID          uuid.UUID
	BookingID   uuid.UUID
	AmountCents int64
	Status      PaymentStatus
	ProviderRef string
	RefundCents int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewPendingPayment(bookingID uuid.UUID, amountCents int64, now time.Time) *Payment {
	return &Payment{
		ID:          uuid.New(),
		BookingID:   bookingID,
		AmountCents: amountCents,
		Status:      PaymentPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
