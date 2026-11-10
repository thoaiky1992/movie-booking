// Package usecase holds application logic. It depends only on the domain and on
// the port interfaces declared here; adapters implement those ports.
package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"movie_booking/internal/domain"
)

// Clock is injected so time-dependent rules (TTL, refund windows) are testable.
type Clock interface{ Now() time.Time }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// Page is a normalised offset-pagination request.
type Page struct {
	Number int
	Size   int
}

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

func NewPage(number, size int) Page {
	if number < 1 {
		number = 1
	}
	if size < 1 {
		size = DefaultPageSize
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	return Page{Number: number, Size: size}
}

func (p Page) Offset() int { return (p.Number - 1) * p.Size }
func (p Page) Limit() int  { return p.Size }

// Paged is a page of results plus the total count for pagination metadata.
type Paged[T any] struct {
	Items []T
	Total int
	Page  Page
}

// ---- Catalog ports --------------------------------------------------------

type MovieFilter struct {
	Query string
	Genre string
	Page  Page
}

type MovieRepository interface {
	List(ctx context.Context, f MovieFilter) ([]domain.Movie, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Movie, error)
}

type CinemaRepository interface {
	List(ctx context.Context, p Page) ([]domain.Cinema, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Cinema, error)
	ListHalls(ctx context.Context, cinemaID uuid.UUID) ([]domain.Hall, error)
}

type ShowtimeFilter struct {
	MovieID  *uuid.UUID
	CinemaID *uuid.UUID
	From     *time.Time
	To       *time.Time
	Page     Page
}

type ShowtimeRepository interface {
	List(ctx context.Context, f ShowtimeFilter) ([]domain.ShowtimeView, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Showtime, error)
	// SeatMap returns every seat in the showtime's hall with its live status.
	// A HELD seat whose hold deadline has passed is reported as AVAILABLE.
	SeatMap(ctx context.Context, showtimeID uuid.UUID, now time.Time) ([]domain.SeatStatus, error)
}

type SeatRepository interface {
	// GetByIDs returns the seats with the given IDs that belong to hallID.
	GetByIDs(ctx context.Context, hallID uuid.UUID, ids []uuid.UUID) ([]domain.Seat, error)
}

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// ---- Booking ports --------------------------------------------------------

// BookingDetail is a read model with everything needed to render or notify.
type BookingDetail struct {
	Booking    domain.Booking
	MovieTitle string
	CinemaName string
	HallName   string
	StartsAt   time.Time
	UserEmail  string
	UserName   string
	Payment    *domain.Payment // most recent payment, if any
}

// RefundTarget identifies a payment whose money must go back to the customer.
type RefundTarget struct {
	PaymentID   uuid.UUID
	ProviderRef string
	AmountCents int64
}

// CancelOutcome describes what the cancel transaction released.
type CancelOutcome struct {
	ShowtimeID uuid.UUID
	SeatIDs    []uuid.UUID
	Refund     *RefundTarget // nil when nothing was paid
}

// ExpiredHold describes a hold the sweeper just expired.
type ExpiredHold struct {
	BookingID  uuid.UUID
	ShowtimeID uuid.UUID
	SeatIDs    []uuid.UUID
}

type BookingRepository interface {
	// CreateHold inserts the booking and its seats in one transaction. It
	// first reclaims seats held by already-expired bookings. Returns
	// domain.ErrSeatUnavailable if a live booking owns any requested seat.
	CreateHold(ctx context.Context, b *domain.Booking, now time.Time) error
	GetDetail(ctx context.Context, id uuid.UUID) (*BookingDetail, error)

	// BeginPayment locks the booking and records a PENDING payment.
	BeginPayment(ctx context.Context, bookingID uuid.UUID, p *domain.Payment, now time.Time) error
	// CompletePayment marks the payment SUCCEEDED, the booking CONFIRMED and
	// writes the confirmation event to the outbox, atomically. Returns
	// domain.ErrNotConfirmable if the hold expired or the booking was cancelled
	// while the charge was in flight; the caller must then refund.
	CompletePayment(ctx context.Context, bookingID, paymentID uuid.UUID, providerRef string, now time.Time, evt domain.BookingEvent) error
	FailPayment(ctx context.Context, paymentID uuid.UUID, now time.Time) error
	// RequestRefund moves a PENDING or SUCCEEDED payment to REFUND_PENDING.
	RequestRefund(ctx context.Context, paymentID uuid.UUID, providerRef string, amountCents int64, now time.Time) error
	CompleteRefund(ctx context.Context, paymentID uuid.UUID, now time.Time) error
	ListRefundPending(ctx context.Context, limit int) ([]RefundTarget, error)

	// CancelBooking cancels a HELD or CONFIRMED booking, releases its seats,
	// marks the payment for refund and writes the outbox event, atomically.
	CancelBooking(ctx context.Context, bookingID uuid.UUID, now time.Time, refundCents int64, evt domain.BookingEvent) (*CancelOutcome, error)

	// ExpireHolds expires up to limit HELD bookings past their deadline and
	// releases their seats. Rows are locked with SKIP LOCKED so sweepers can
	// run in parallel.
	ExpireHolds(ctx context.Context, now time.Time, limit int) ([]ExpiredHold, error)
}

// SeatGate is the fast, Redis-backed first line of defence against
// concurrent holds. It is an accelerator: the database remains authoritative.
type SeatGate interface {
	// Acquire atomically locks all seats for bookingID with the given TTL.
	// Returns domain.ErrSeatUnavailable if any seat is already locked.
	Acquire(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID, ttl time.Duration) error
	// Release frees the seats, but only those still owned by bookingID.
	Release(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID) error
	// Confirm extends ownership of the seats until the given instant.
	Confirm(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID, until time.Time) error
}

type ChargeRequest struct {
	IdempotencyKey string
	AmountCents    int64
	Currency       string
	Description    string
}

type ChargeResult struct {
	ProviderRef string
}

type RefundRequest struct {
	IdempotencyKey string
	ProviderRef    string
	AmountCents    int64
}

// PaymentGateway abstracts the PSP. Implementations MUST honour IdempotencyKey
// so that retries never double-charge or double-refund.
type PaymentGateway interface {
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
	Refund(ctx context.Context, req RefundRequest) error
}

// ---- Messaging ports ------------------------------------------------------

type EventPublisher interface {
	Publish(ctx context.Context, topic, key string, value []byte) error
}

type InlineImage struct {
	ContentID   string
	ContentType string
	Filename    string
	Data        []byte
}

type EmailMessage struct {
	To       string
	Subject  string
	HTMLBody string
	Inline   []InlineImage
}

type Mailer interface {
	Send(ctx context.Context, msg EmailMessage) error
}

type TicketComposer interface {
	Compose(evt domain.BookingEvent) (EmailMessage, error)
}

// Dedupe provides at-most-once side effects for at-least-once consumers.
type Dedupe interface {
	// Claim returns true if the key was newly claimed.
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

// ---- Rating ports ---------------------------------------------------------

// RatingProvider is a third-party source. Fetch returns zero values and nil
// error when the partner has no data for the movie.
type RatingProvider interface {
	Name() string
	Fetch(ctx context.Context, movie domain.Movie) ([]domain.RatingValue, error)
}

type RatingStore interface {
	Get(ctx context.Context, movieID uuid.UUID) (*domain.RatingSnapshot, error)
	Upsert(ctx context.Context, snap domain.RatingSnapshot) error
	MarkAttempt(ctx context.Context, movieID uuid.UUID, at time.Time) error
	// ListStale returns movies not attempted since attemptedBefore, oldest first.
	ListStale(ctx context.Context, attemptedBefore time.Time, limit int) ([]domain.Movie, error)
}

type RatingCache interface {
	Get(ctx context.Context, movieID uuid.UUID) (*domain.RatingSnapshot, error)
	Set(ctx context.Context, snap domain.RatingSnapshot, ttl time.Duration) error
}

// Readiness is implemented by infrastructure that can be pinged (DB, Redis).
type Readiness interface {
	Ping(ctx context.Context) error
}
