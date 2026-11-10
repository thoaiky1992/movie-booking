package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"movie_booking/internal/domain"
)

type BookingConfig struct {
	HoldTTL      time.Duration
	MaxSeats     int
	Currency     string
	SeatKeepTail time.Duration // keep Redis seat locks this long after showtime end
}

type BookingService struct {
	bookings  BookingRepository
	showtimes ShowtimeRepository
	seats     SeatRepository
	gate      SeatGate
	gateway   PaymentGateway
	clock     Clock
	cfg       BookingConfig
	log       *slog.Logger
}

func NewBookingService(
	bookings BookingRepository,
	showtimes ShowtimeRepository,
	seats SeatRepository,
	gate SeatGate,
	gateway PaymentGateway,
	clock Clock,
	cfg BookingConfig,
	log *slog.Logger,
) *BookingService {
	return &BookingService{
		bookings: bookings, showtimes: showtimes, seats: seats,
		gate: gate, gateway: gateway, clock: clock, cfg: cfg, log: log,
	}
}

// Hold locks the selected seats for the user for cfg.HoldTTL.
//
// Two layers guard the seats, and the order matters:
//  1. Redis Lua gate: atomically claims every seat or none. Rejects hot-seat
//     contention in-memory, without touching PostgreSQL.
//  2. PostgreSQL transaction: inserts the booking and its seat rows. The partial
//     unique index is the final arbiter, so correctness never depends on Redis.
//
// If the DB step fails, the Redis claim is released (compare-and-delete).
func (s *BookingService) Hold(ctx context.Context, userID, showtimeID uuid.UUID, seatIDs []uuid.UUID) (*domain.Booking, error) {
	now := s.clock.Now()

	show, err := s.showtimes.GetByID(ctx, showtimeID)
	if err != nil {
		return nil, fmt.Errorf("load showtime: %w", err)
	}
	unique := uniqueUUIDs(seatIDs)
	if len(unique) == 0 {
		return nil, fmt.Errorf("%w: seat_ids is required", domain.ErrInvalidInput)
	}
	seats, err := s.seats.GetByIDs(ctx, show.HallID, unique)
	if err != nil {
		return nil, fmt.Errorf("load seats: %w", err)
	}
	if len(seats) != len(unique) {
		return nil, fmt.Errorf("%w: one or more seats do not belong to this showtime", domain.ErrInvalidInput)
	}

	booking, err := domain.NewHold(userID, *show, seats, now, s.cfg.HoldTTL, s.cfg.MaxSeats)
	if err != nil {
		return nil, err
	}

	ids := booking.SeatIDs()
	if err := s.gate.Acquire(ctx, show.ID, ids, booking.ID, s.cfg.HoldTTL); err != nil {
		if errors.Is(err, domain.ErrSeatUnavailable) {
			return nil, err
		}
		// Redis is an accelerator, not the source of truth. Degrade to the DB guard.
		s.log.Warn("seat gate unavailable, relying on database guard", "err", err)
	}

	if err := s.bookings.CreateHold(ctx, booking, now); err != nil {
		s.releaseGate(ctx, show.ID, ids, booking.ID)
		return nil, err
	}
	return booking, nil
}

// Get returns a booking owned by userID. Foreign bookings are reported as not
// found so that booking IDs cannot be probed.
func (s *BookingService) Get(ctx context.Context, userID, bookingID uuid.UUID) (*BookingDetail, error) {
	d, err := s.bookings.GetDetail(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if d.Booking.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return d, nil
}

// Pay charges the customer and confirms the booking.
//
// The charge happens outside any DB transaction (no locks held across a
// network call). Its outcome is then reconciled under a row lock. If the hold
// expired or the booking was cancelled during the charge, the money is refunded.
func (s *BookingService) Pay(ctx context.Context, userID, bookingID uuid.UUID) (*BookingDetail, error) {
	now := s.clock.Now()
	detail, err := s.Get(ctx, userID, bookingID)
	if err != nil {
		return nil, err
	}
	if err := detail.Booking.IsPayable(now); err != nil {
		return nil, err
	}

	payment := domain.NewPendingPayment(bookingID, detail.Booking.TotalCents, now)
	if err := s.bookings.BeginPayment(ctx, bookingID, payment, now); err != nil {
		return nil, err
	}

	charge, err := s.gateway.Charge(ctx, ChargeRequest{
		IdempotencyKey: payment.ID.String(),
		AmountCents:    payment.AmountCents,
		Currency:       s.cfg.Currency,
		Description:    "Movie ticket " + detail.Booking.TicketCode,
	})
	if err != nil {
		if mErr := s.bookings.FailPayment(ctx, payment.ID, s.clock.Now()); mErr != nil {
			s.log.Error("mark payment failed", "payment_id", payment.ID, "err", mErr)
		}
		if errors.Is(err, domain.ErrPaymentDeclined) {
			return nil, err
		}
		return nil, fmt.Errorf("charge payment: %w", err)
	}

	confirmedAt := s.clock.Now()
	evt := s.newEvent(domain.EventBookingConfirmed, detail, 0, confirmedAt)
	err = s.bookings.CompletePayment(ctx, bookingID, payment.ID, charge.ProviderRef, confirmedAt, evt)
	if err != nil {
		// The customer was charged but the booking was not confirmed. Never keep
		// money for nothing: request a refund for this payment.
		s.compensateCharge(ctx, payment, charge.ProviderRef)
		if errors.Is(err, domain.ErrNotConfirmable) {
			return nil, domain.ErrHoldExpired
		}
		return nil, fmt.Errorf("complete payment: %w", err)
	}

	// Best effort: pin the Redis locks to the sold seats. DB already has the truth.
	until := detail.StartsAt.Add(s.cfg.SeatKeepTail)
	if err := s.gate.Confirm(ctx, detail.Booking.ShowtimeID, detail.Booking.SeatIDs(), bookingID, until); err != nil {
		s.log.Warn("extend seat locks after payment failed", "booking_id", bookingID, "err", err)
	}

	return s.bookings.GetDetail(ctx, bookingID)
}

// CancelResult summarises a cancellation for the caller.
type CancelResult struct {
	BookingID    uuid.UUID
	Status       domain.BookingStatus
	RefundCents  int64
	RefundStatus string // NONE | PENDING | REFUNDED
}

// Cancel releases the seats. A confirmed booking is refunded according to the
// cancellation policy (domain.CancellationRefund).
func (s *BookingService) Cancel(ctx context.Context, userID, bookingID uuid.UUID) (*CancelResult, error) {
	now := s.clock.Now()
	detail, err := s.Get(ctx, userID, bookingID)
	if err != nil {
		return nil, err
	}
	refund, err := domain.CancellationRefund(detail.Booking, detail.StartsAt, now)
	if err != nil {
		return nil, err
	}

	evt := s.newEvent(domain.EventBookingCancelled, detail, refund, now)
	outcome, err := s.bookings.CancelBooking(ctx, bookingID, now, refund, evt)
	if err != nil {
		return nil, err
	}

	s.releaseGate(ctx, outcome.ShowtimeID, outcome.SeatIDs, bookingID)

	result := &CancelResult{
		BookingID:    bookingID,
		Status:       domain.BookingCancelled,
		RefundCents:  refund,
		RefundStatus: "NONE",
	}
	if outcome.Refund != nil {
		result.RefundStatus = "PENDING"
		if s.tryRefund(ctx, *outcome.Refund) {
			result.RefundStatus = "REFUNDED"
		}
	}
	return result, nil
}

// ExpireHolds is run periodically by the worker. It is the durable backstop
// for hold TTLs: Redis keys also expire, but the database is authoritative.
func (s *BookingService) ExpireHolds(ctx context.Context, limit int) (int, error) {
	expired, err := s.bookings.ExpireHolds(ctx, s.clock.Now(), limit)
	if err != nil {
		return 0, err
	}
	for _, h := range expired {
		s.releaseGate(ctx, h.ShowtimeID, h.SeatIDs, h.BookingID)
	}
	return len(expired), nil
}

// RetryPendingRefunds re-attempts refunds that failed earlier. Idempotency keys
// (the payment ID) make retries safe even if the first attempt actually succeeded.
func (s *BookingService) RetryPendingRefunds(ctx context.Context, limit int) (int, error) {
	pending, err := s.bookings.ListRefundPending(ctx, limit)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, r := range pending {
		if s.tryRefund(ctx, r) {
			done++
		}
	}
	return done, nil
}

// compensateCharge refunds a charge that could not be recorded as a confirmed
// booking. If even the refund request cannot be stored, the payment stays
// PENDING and is logged loudly for manual reconciliation against the PSP.
func (s *BookingService) compensateCharge(ctx context.Context, p *domain.Payment, providerRef string) {
	if err := s.bookings.RequestRefund(ctx, p.ID, providerRef, p.AmountCents, s.clock.Now()); err != nil {
		s.log.Error("RECONCILE: charge recorded at PSP but refund could not be requested",
			"payment_id", p.ID, "provider_ref", providerRef, "amount_cents", p.AmountCents, "err", err)
		return
	}
	s.log.Warn("charge compensated with refund", "payment_id", p.ID, "booking_id", p.BookingID)
	s.tryRefund(ctx, RefundTarget{PaymentID: p.ID, ProviderRef: providerRef, AmountCents: p.AmountCents})
}

func (s *BookingService) tryRefund(ctx context.Context, r RefundTarget) bool {
	err := s.gateway.Refund(ctx, RefundRequest{
		IdempotencyKey: r.PaymentID.String(),
		ProviderRef:    r.ProviderRef,
		AmountCents:    r.AmountCents,
	})
	if err != nil {
		s.log.Error("refund failed, left REFUND_PENDING for retry", "payment_id", r.PaymentID, "err", err)
		return false
	}
	if err := s.bookings.CompleteRefund(ctx, r.PaymentID, s.clock.Now()); err != nil {
		s.log.Error("refund sent but status update failed", "payment_id", r.PaymentID, "err", err)
		return false
	}
	return true
}

func (s *BookingService) releaseGate(ctx context.Context, showtimeID uuid.UUID, seatIDs []uuid.UUID, bookingID uuid.UUID) {
	if err := s.gate.Release(ctx, showtimeID, seatIDs, bookingID); err != nil {
		s.log.Warn("release seat locks failed; TTL will reclaim them", "booking_id", bookingID, "err", err)
	}
}

func (s *BookingService) newEvent(typ string, d *BookingDetail, refund int64, now time.Time) domain.BookingEvent {
	seats := make([]string, 0, len(d.Booking.Seats))
	for _, sl := range d.Booking.Seats {
		seats = append(seats, fmt.Sprintf("%s%d", sl.RowLabel, sl.SeatNumber))
	}
	return domain.BookingEvent{
		EventID:     uuid.NewString(),
		Type:        typ,
		BookingID:   d.Booking.ID.String(),
		OccurredAt:  now,
		TicketCode:  d.Booking.TicketCode,
		Recipient:   domain.Recipient{Email: d.UserEmail, Name: d.UserName},
		MovieTitle:  d.MovieTitle,
		CinemaName:  d.CinemaName,
		HallName:    d.HallName,
		StartsAt:    d.StartsAt,
		Seats:       seats,
		TotalCents:  d.Booking.TotalCents,
		RefundCents: refund,
		Currency:    s.cfg.Currency,
	}
}

func uniqueUUIDs(in []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(in))
	out := make([]uuid.UUID, 0, len(in))
	for _, id := range in {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
