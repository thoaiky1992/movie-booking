package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"movie-booking/internal/domain"
)

type BookingRepository struct {
	db *sql.DB
}

func NewBookingRepository(db *sql.DB) *BookingRepository {
	return &BookingRepository{db: db}
}

func (r *BookingRepository) HoldSeats(
	ctx context.Context,
	userID int64,
	showtimeID int64,
	seatIDs []int64,
	holdToken string,
	ttl time.Duration,
) (_ *domain.SeatHold, returnErr error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin seat hold transaction: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = tx.Rollback()
		}
	}()

	var (
		roomID    int64
		basePrice string
		started   bool
	)
	err = tx.QueryRowContext(ctx, `
		SELECT room_id, base_price::text, starts_at <= NOW()
		FROM showtimes
		WHERE id = $1`, showtimeID).Scan(&roomID, &basePrice, &started)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrShowtimeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load showtime for seat hold: %w", err)
	}
	if started {
		return nil, domain.ErrShowtimeStarted
	}

	var validSeatCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM seats
		WHERE room_id = $1 AND id = ANY($2::bigint[])`, roomID, seatIDs).
		Scan(&validSeatCount); err != nil {
		return nil, fmt.Errorf("validate seats: %w", err)
	}
	if validSeatCount != len(seatIDs) {
		return nil, domain.ErrInvalidSeatHold
	}

	hold := &domain.SeatHold{
		ShowtimeID: showtimeID,
		SeatIDs:    append([]int64(nil), seatIDs...),
		HoldToken:  holdToken,
		Status:     "PENDING",
	}
	ttlMilliseconds := ttl.Milliseconds()
	err = tx.QueryRowContext(ctx, `
		INSERT INTO bookings (
			user_id, showtime_id, status, total_amount, expires_at, hold_token
		)
		VALUES (
			$1, $2, 'PENDING', $3::numeric * $4,
			NOW() + ($5 * INTERVAL '1 millisecond'), $6
		)
		RETURNING id, total_amount::text, expires_at`,
		userID,
		showtimeID,
		basePrice,
		len(seatIDs),
		ttlMilliseconds,
		holdToken,
	).Scan(&hold.BookingID, &hold.TotalAmount, &hold.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("create pending booking: %w", err)
	}

	// PostgreSQL's unique index arbitrates concurrent inserts. ON CONFLICT may
	// replace only an already-expired HELD claim; BOOKED and live HELD claims
	// cannot be overwritten. A short RETURNING result rolls the whole tx back.
	rows, err := tx.QueryContext(ctx, `
		INSERT INTO seat_claims (
			showtime_id, seat_id, room_id, booking_id, status, expires_at
		)
		SELECT $1, requested.seat_id, $2, $3, 'HELD', $4
		FROM unnest($5::bigint[]) AS requested(seat_id)
		ON CONFLICT (showtime_id, seat_id) DO UPDATE
		SET room_id = EXCLUDED.room_id,
			booking_id = EXCLUDED.booking_id,
			status = 'HELD',
			expires_at = EXCLUDED.expires_at,
			created_at = NOW()
		WHERE seat_claims.status = 'HELD'
			AND seat_claims.expires_at <= NOW()
		RETURNING seat_id`,
		showtimeID,
		roomID,
		hold.BookingID,
		hold.ExpiresAt,
		seatIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("claim seats: %w", err)
	}
	defer rows.Close() // ensure rows are closed even if we return early

	claimed := 0
	for rows.Next() {
		claimed++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read claimed seats: %w", err)
	}
	if claimed != len(seatIDs) {
		return nil, domain.ErrSeatsUnavailable
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO booking_items (booking_id, showtime_id, seat_id, price)
		SELECT $1, $2, requested.seat_id, $3::numeric
		FROM unnest($4::bigint[]) AS requested(seat_id)`,
		hold.BookingID,
		showtimeID,
		basePrice,
		seatIDs,
	); err != nil {
		return nil, fmt.Errorf("create booking items: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit seat hold: %w", err)
	}
	return hold, nil
}

func (r *BookingRepository) ExpirePendingHolds(
	ctx context.Context,
	batchSize int,
) (_ []domain.ReleasedSeat, returnErr error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin expiry transaction: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = tx.Rollback()
		}
	}()

	// SKIP LOCKED lets multiple worker replicas drain different batches without
	// a leader election or waiting on one another.
	rows, err := tx.QueryContext(ctx, `
		SELECT b.id
		FROM bookings AS b
		WHERE b.status = 'PENDING' AND b.expires_at <= NOW()
		ORDER BY b.expires_at, b.id
		LIMIT $1
		FOR UPDATE OF b SKIP LOCKED`, batchSize)
	if err != nil {
		return nil, fmt.Errorf("select expired holds: %w", err)
	}
	defer rows.Close() // defer rows.Close() // ensure rows are closed even if we return early

	bookingIDs := make([]int64, 0, batchSize)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan expired booking: %w", err)
		}
		bookingIDs = append(bookingIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read expired bookings: %w", err)
	}
	if len(bookingIDs) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit empty expiry batch: %w", err)
		}
		return []domain.ReleasedSeat{}, nil
	}

	releasedRows, err := tx.QueryContext(ctx, `
		DELETE FROM seat_claims AS sc
		USING bookings AS b
		WHERE sc.booking_id = b.id
			AND sc.status = 'HELD'
			AND b.id = ANY($1::bigint[])
		RETURNING sc.showtime_id, sc.seat_id, COALESCE(b.hold_token, '')`,
		bookingIDs)
	if err != nil {
		return nil, fmt.Errorf("release expired seat claims: %w", err)
	}
	defer releasedRows.Close() // ensure releasedRows are closed even if we return early

	released := make([]domain.ReleasedSeat, 0)
	for releasedRows.Next() {
		var seat domain.ReleasedSeat
		if err := releasedRows.Scan(&seat.ShowtimeID, &seat.SeatID, &seat.HoldToken); err != nil {
			return nil, fmt.Errorf("scan released seat: %w", err)
		}
		released = append(released, seat)
	}
	if err := releasedRows.Err(); err != nil {
		return nil, fmt.Errorf("read released seats: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE bookings
		SET status = 'EXPIRED'
		WHERE id = ANY($1::bigint[]) AND status = 'PENDING'`, bookingIDs); err != nil {
		return nil, fmt.Errorf("expire bookings: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit expiry batch: %w", err)
	}
	return released, nil
}
