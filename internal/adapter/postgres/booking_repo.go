package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

// BookingRepo owns every state transition of bookings, booking_seats and payments.
// Each transition runs in one transaction, so seat state and money state cannot diverge.
type BookingRepo struct {
	pool        *pgxpool.Pool
	eventsTopic string
}

func NewBookingRepo(pool *pgxpool.Pool, eventsTopic string) *BookingRepo {
	return &BookingRepo{pool: pool, eventsTopic: eventsTopic}
}

var _ usecase.BookingRepository = (*BookingRepo)(nil)

// CreateHold is the database side of the anti double-booking guarantee.
//
// The partial unique index uq_booking_seats_active makes it impossible for two
// live bookings to hold the same seat, whatever Redis did or did not do. Before
// inserting, expired holds on the requested seats are reclaimed inline, so a
// customer never waits for the sweeper to free a seat whose TTL already passed.
func (r *BookingRepo) CreateHold(ctx context.Context, b *domain.Booking, now time.Time) error {
	seatIDs := uuidStrings(b.SeatIDs())
	prices := make([]int64, len(b.Seats))
	for i, s := range b.Seats {
		prices[i] = s.PriceCents
	}

	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			WITH stale AS (
				SELECT DISTINCT bs.booking_id
				FROM booking_seats bs
				JOIN bookings bk ON bk.id = bs.booking_id
				WHERE bs.showtime_id = $1
				  AND bs.seat_id = ANY($2::uuid[])
				  AND bs.status = 'HELD'
				  AND bk.status = 'HELD'
				  AND bk.expires_at <= $3
			), expired AS (
				UPDATE bookings SET status = 'EXPIRED', updated_at = $3
				WHERE id IN (SELECT booking_id FROM stale) AND status = 'HELD'
				RETURNING id
			)
			UPDATE booking_seats SET status = 'RELEASED'
			WHERE booking_id IN (SELECT id FROM expired) AND status = 'HELD'`,
			b.ShowtimeID, seatIDs, now); err != nil {
			return fmt.Errorf("reclaim expired holds: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO bookings (id, user_id, showtime_id, status, total_cents, ticket_code, expires_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'HELD', $4, $5, $6, $7, $7)`,
			b.ID, b.UserID, b.ShowtimeID, b.TotalCents, b.TicketCode, b.ExpiresAt, now); err != nil {
			return fmt.Errorf("insert booking: %w", err)
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO booking_seats (booking_id, showtime_id, seat_id, status, price_cents)
			SELECT $1, $2, t.seat_id, 'HELD', t.price
			FROM unnest($3::uuid[], $4::bigint[]) AS t(seat_id, price)`,
			b.ID, b.ShowtimeID, seatIDs, prices)
		if isUniqueViolation(err, "uq_booking_seats_active") {
			return fmt.Errorf("%w: one or more seats were just taken", domain.ErrSeatUnavailable)
		}
		if err != nil {
			return fmt.Errorf("insert booking seats: %w", err)
		}
		return nil
	})
}

func (r *BookingRepo) GetDetail(ctx context.Context, id uuid.UUID) (*usecase.BookingDetail, error) {
	var d usecase.BookingDetail
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT b.id, b.user_id, b.showtime_id, b.status, b.total_cents, b.ticket_code,
		       b.expires_at, b.confirmed_at, b.cancelled_at, b.created_at, b.updated_at,
		       m.title, c.name, h.name, s.starts_at, u.email, u.full_name
		FROM bookings b
		JOIN showtimes s ON s.id = b.showtime_id
		JOIN movies m    ON m.id = s.movie_id
		JOIN halls h     ON h.id = s.hall_id
		JOIN cinemas c   ON c.id = h.cinema_id
		JOIN users u     ON u.id = b.user_id
		WHERE b.id = $1`, id).Scan(
		&d.Booking.ID, &d.Booking.UserID, &d.Booking.ShowtimeID, &status, &d.Booking.TotalCents, &d.Booking.TicketCode,
		&d.Booking.ExpiresAt, &d.Booking.ConfirmedAt, &d.Booking.CancelledAt, &d.Booking.CreatedAt, &d.Booking.UpdatedAt,
		&d.MovieTitle, &d.CinemaName, &d.HallName, &d.StartsAt, &d.UserEmail, &d.UserName,
	)
	if err != nil {
		return nil, notFoundOr(err, "booking")
	}
	d.Booking.Status = domain.BookingStatus(status)

	rows, err := r.pool.Query(ctx, `
		SELECT bs.seat_id, s.row_label, s.seat_number, s.seat_type, bs.price_cents
		FROM booking_seats bs
		JOIN seats s ON s.id = bs.seat_id
		WHERE bs.booking_id = $1
		ORDER BY s.row_label, s.seat_number`, id)
	if err != nil {
		return nil, fmt.Errorf("load booking seats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sl domain.SeatLine
		if err := rows.Scan(&sl.SeatID, &sl.RowLabel, &sl.SeatNumber, &sl.SeatType, &sl.PriceCents); err != nil {
			return nil, fmt.Errorf("scan booking seat: %w", err)
		}
		d.Booking.Seats = append(d.Booking.Seats, sl)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	p, err := r.latestPayment(ctx, id)
	if err != nil {
		return nil, err
	}
	d.Payment = p
	return &d, nil
}

func (r *BookingRepo) latestPayment(ctx context.Context, bookingID uuid.UUID) (*domain.Payment, error) {
	var p domain.Payment
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT id, booking_id, amount_cents, status, provider_ref, refund_cents, created_at, updated_at
		FROM payments WHERE booking_id = $1 ORDER BY created_at DESC LIMIT 1`, bookingID).
		Scan(&p.ID, &p.BookingID, &p.AmountCents, &status, &p.ProviderRef, &p.RefundCents, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load payment: %w", err)
	}
	p.Status = domain.PaymentStatus(status)
	return &p, nil
}

// lockBooking takes a row lock on the booking, serialising every state change on it.
func lockBooking(ctx context.Context, tx pgx.Tx, id uuid.UUID) (status string, expiresAt *time.Time, err error) {
	err = tx.QueryRow(ctx, `SELECT status, expires_at FROM bookings WHERE id = $1 FOR UPDATE`, id).
		Scan(&status, &expiresAt)
	return status, expiresAt, notFoundOr(err, "booking")
}

func (r *BookingRepo) BeginPayment(ctx context.Context, bookingID uuid.UUID, p *domain.Payment, now time.Time) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		status, expiresAt, err := lockBooking(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if status != string(domain.BookingHeld) {
			return fmt.Errorf("%w: booking is %s", domain.ErrInvalidState, status)
		}
		if expiresAt == nil || !now.Before(*expiresAt) {
			return domain.ErrHoldExpired
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO payments (id, booking_id, amount_cents, status, provider_ref, refund_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'PENDING', '', 0, $4, $4)`,
			p.ID, bookingID, p.AmountCents, now)
		if isUniqueViolation(err, "uq_payments_open_per_booking") {
			return domain.ErrPaymentInProgress
		}
		if err != nil {
			return fmt.Errorf("insert payment: %w", err)
		}
		return nil
	})
}

func (r *BookingRepo) CompletePayment(ctx context.Context, bookingID, paymentID uuid.UUID, providerRef string, now time.Time, evt domain.BookingEvent) error {
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		status, expiresAt, err := lockBooking(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if status != string(domain.BookingHeld) || expiresAt == nil || !now.Before(*expiresAt) {
			return domain.ErrNotConfirmable
		}

		tag, err := tx.Exec(ctx, `
			UPDATE payments SET status = 'SUCCEEDED', provider_ref = $2, updated_at = $3
			WHERE id = $1 AND status = 'PENDING'`, paymentID, providerRef, now)
		if err != nil {
			return fmt.Errorf("mark payment succeeded: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("%w: payment is not pending", domain.ErrInvalidState)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE bookings SET status = 'CONFIRMED', expires_at = NULL, confirmed_at = $2, updated_at = $2
			WHERE id = $1`, bookingID, now); err != nil {
			return fmt.Errorf("confirm booking: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE booking_seats SET status = 'CONFIRMED' WHERE booking_id = $1 AND status = 'HELD'`, bookingID); err != nil {
			return fmt.Errorf("confirm seats: %w", err)
		}
		return insertOutbox(ctx, tx, r.eventsTopic, evt, now)
	})
}

func (r *BookingRepo) FailPayment(ctx context.Context, paymentID uuid.UUID, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE payments SET status = 'FAILED', updated_at = $2
		WHERE id = $1 AND status = 'PENDING'`, paymentID, now)
	if err != nil {
		return fmt.Errorf("mark payment failed: %w", err)
	}
	return nil
}

func (r *BookingRepo) RequestRefund(ctx context.Context, paymentID uuid.UUID, providerRef string, amountCents int64, now time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE payments SET status = 'REFUND_PENDING', provider_ref = $2, refund_cents = $3, updated_at = $4
		WHERE id = $1 AND status IN ('PENDING', 'SUCCEEDED')`, paymentID, providerRef, amountCents, now)
	if err != nil {
		return fmt.Errorf("request refund: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: payment cannot be refunded in its current state", domain.ErrInvalidState)
	}
	return nil
}

func (r *BookingRepo) CompleteRefund(ctx context.Context, paymentID uuid.UUID, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE payments SET status = 'REFUNDED', updated_at = $2
		WHERE id = $1 AND status = 'REFUND_PENDING'`, paymentID, now)
	if err != nil {
		return fmt.Errorf("complete refund: %w", err)
	}
	return nil
}

func (r *BookingRepo) ListRefundPending(ctx context.Context, limit int) ([]usecase.RefundTarget, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, provider_ref, refund_cents FROM payments
		WHERE status = 'REFUND_PENDING'
		ORDER BY updated_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending refunds: %w", err)
	}
	defer rows.Close()

	var out []usecase.RefundTarget
	for rows.Next() {
		var t usecase.RefundTarget
		if err := rows.Scan(&t.PaymentID, &t.ProviderRef, &t.AmountCents); err != nil {
			return nil, fmt.Errorf("scan pending refund: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *BookingRepo) CancelBooking(ctx context.Context, bookingID uuid.UUID, now time.Time, refundCents int64, evt domain.BookingEvent) (*usecase.CancelOutcome, error) {
	var out usecase.CancelOutcome
	err := withTx(ctx, r.pool, func(tx pgx.Tx) error {
		status, _, err := lockBooking(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if status != string(domain.BookingHeld) && status != string(domain.BookingConfirmed) {
			return fmt.Errorf("%w: booking is %s", domain.ErrInvalidState, status)
		}

		var pendingPayment bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM payments WHERE booking_id = $1 AND status = 'PENDING')`, bookingID).
			Scan(&pendingPayment); err != nil {
			return fmt.Errorf("check in-flight payment: %w", err)
		}
		if pendingPayment {
			return domain.ErrPaymentInProgress
		}

		if err := tx.QueryRow(ctx, `
			UPDATE bookings SET status = 'CANCELLED', expires_at = NULL, cancelled_at = $2, updated_at = $2
			WHERE id = $1 RETURNING showtime_id`, bookingID, now).Scan(&out.ShowtimeID); err != nil {
			return fmt.Errorf("cancel booking: %w", err)
		}

		rows, err := tx.Query(ctx, `
			UPDATE booking_seats SET status = 'RELEASED'
			WHERE booking_id = $1 AND status IN ('HELD', 'CONFIRMED')
			RETURNING seat_id`, bookingID)
		if err != nil {
			return fmt.Errorf("release seats: %w", err)
		}
		for rows.Next() {
			var seatID uuid.UUID
			if err := rows.Scan(&seatID); err != nil {
				rows.Close()
				return fmt.Errorf("scan released seat: %w", err)
			}
			out.SeatIDs = append(out.SeatIDs, seatID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		var paymentID uuid.UUID
		var providerRef string
		err = tx.QueryRow(ctx, `
			SELECT id, provider_ref FROM payments
			WHERE booking_id = $1 AND status = 'SUCCEEDED'
			FOR UPDATE`, bookingID).Scan(&paymentID, &providerRef)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if refundCents > 0 {
				return fmt.Errorf("%w: no successful payment to refund", domain.ErrInvalidState)
			}
		case err != nil:
			return fmt.Errorf("load payment for cancel: %w", err)
		case refundCents > 0:
			if _, err := tx.Exec(ctx, `
				UPDATE payments SET status = 'REFUND_PENDING', refund_cents = $2, updated_at = $3 WHERE id = $1`,
				paymentID, refundCents, now); err != nil {
				return fmt.Errorf("mark refund pending: %w", err)
			}
			out.Refund = &usecase.RefundTarget{PaymentID: paymentID, ProviderRef: providerRef, AmountCents: refundCents}
		default:
			if _, err := tx.Exec(ctx, `
				UPDATE payments SET status = 'NO_REFUND', updated_at = $2 WHERE id = $1`, paymentID, now); err != nil {
				return fmt.Errorf("mark no refund: %w", err)
			}
		}

		return insertOutbox(ctx, tx, r.eventsTopic, evt, now)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ExpireHolds locks due bookings with SKIP LOCKED, so concurrent sweepers never
// block each other and never process the same booking twice.
func (r *BookingRepo) ExpireHolds(ctx context.Context, now time.Time, limit int) ([]usecase.ExpiredHold, error) {
	rows, err := r.pool.Query(ctx, `
		WITH due AS (
			SELECT id FROM bookings
			WHERE status = 'HELD' AND expires_at <= $1
			ORDER BY expires_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		), expired AS (
			UPDATE bookings b SET status = 'EXPIRED', updated_at = $1
			FROM due
			WHERE b.id = due.id
			RETURNING b.id, b.showtime_id
		), released AS (
			UPDATE booking_seats bs SET status = 'RELEASED'
			FROM expired e
			WHERE bs.booking_id = e.id AND bs.status = 'HELD'
			RETURNING bs.booking_id, bs.seat_id
		)
		SELECT e.id, e.showtime_id, rel.seat_id
		FROM expired e
		LEFT JOIN released rel ON rel.booking_id = e.id`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("expire holds: %w", err)
	}
	defer rows.Close()

	index := map[uuid.UUID]int{}
	var out []usecase.ExpiredHold
	for rows.Next() {
		var bookingID, showtimeID uuid.UUID
		var seatID *uuid.UUID
		if err := rows.Scan(&bookingID, &showtimeID, &seatID); err != nil {
			return nil, fmt.Errorf("scan expired hold: %w", err)
		}
		i, ok := index[bookingID]
		if !ok {
			i = len(out)
			index[bookingID] = i
			out = append(out, usecase.ExpiredHold{BookingID: bookingID, ShowtimeID: showtimeID})
		}
		if seatID != nil {
			out[i].SeatIDs = append(out[i].SeatIDs, *seatID)
		}
	}
	return out, rows.Err()
}

func insertOutbox(ctx context.Context, tx pgx.Tx, topic string, evt domain.BookingEvent, now time.Time) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, topic, message_key, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		evt.EventID, topic, evt.BookingID, evt.Type, string(payload), now)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
