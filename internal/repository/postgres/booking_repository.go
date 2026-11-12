package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
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
		// The showtime does not exist, so we cannot hold seats for it.
		return nil, domain.ErrShowtimeNotFound
	}
	if err != nil {
		// The showtime exists, but we failed to load it for some reason.
		return nil, fmt.Errorf("load showtime for seat hold: %w", err)
	}
	if started {
		// The showtime has already started, so we cannot hold seats for it.
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
		// The requested seats do not all exist in the showtime's room, so we cannot hold them.
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
		// The booking could not be created, so we cannot hold the seats.
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
		// The seat claims could not be created, so we cannot hold the seats.
		return nil, fmt.Errorf("claim seats: %w", err)
	}
	defer rows.Close() // ensure rows are closed even if we return early

	claimed := 0
	for rows.Next() {
		claimed++
	}
	if err := rows.Err(); err != nil {
		// The seat claims could not be read, so we cannot hold the seats.
		return nil, fmt.Errorf("read claimed seats: %w", err)
	}
	if claimed != len(seatIDs) {
		// The seat claims could not be created for all requested seats, so we cannot hold them.
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
		// The booking items could not be created, so we cannot hold the seats.
		return nil, fmt.Errorf("create booking items: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit seat hold: %w", err)
	}
	return hold, nil
}

func (r *BookingRepository) ConfirmBooking(
	ctx context.Context,
	userID, bookingID int64,
) (_ *domain.ConfirmedBooking, returnErr error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin booking confirmation transaction: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = tx.Rollback()
		}
	}()

	confirmed := &domain.ConfirmedBooking{
		BookingID: bookingID,
		Status:    "CONFIRMED",
	}
	var (
		status      string
		unexpired   bool
		confirmedAt sql.NullTime
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			b.showtime_id,
			b.status,
			b.total_amount::text,
			b.expires_at > NOW(),
			b.confirmed_at,
			COALESCE(b.hold_token, ''),
			u.email
		FROM bookings AS b
		JOIN users AS u ON u.id = b.user_id
		WHERE b.id = $1 AND b.user_id = $2
		FOR UPDATE OF b`, bookingID, userID).Scan(
		&confirmed.ShowtimeID,
		&status,
		&confirmed.TotalAmount,
		&unexpired,
		&confirmedAt,
		&confirmed.HoldToken,
		&confirmed.RecipientEmail,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// The booking does not exist or does not belong to the user, so we cannot confirm it.
		return nil, domain.ErrBookingNotFound
	}
	if err != nil {
		// The booking exists and belongs to the user, but we failed to load it for some reason.
		return nil, fmt.Errorf("load booking for confirmation: %w", err)
	}
	if status == "CONFIRMED" && confirmedAt.Valid {
		confirmed.ConfirmedAt = confirmedAt.Time
		seatRows, err := tx.QueryContext(ctx, `
			SELECT seat_id
			FROM booking_items
			WHERE booking_id = $1
			ORDER BY seat_id`, bookingID)
		if err != nil {
			// The booking is already confirmed, but we failed to load the confirmed seats for some reason.
			return nil, fmt.Errorf("load confirmed booking seats: %w", err)
		}
		defer seatRows.Close()
		for seatRows.Next() {
			var seatID int64
			if err := seatRows.Scan(&seatID); err != nil {
				// The booking is already confirmed, but we failed to read the confirmed seats for some reason.
				return nil, fmt.Errorf("scan existing confirmed booking seat: %w", err)
			}
			confirmed.SeatIDs = append(confirmed.SeatIDs, seatID)
		}
		if err := seatRows.Err(); err != nil {
			// The booking is already confirmed, but we failed to read the confirmed seats for some reason.
			return nil, fmt.Errorf("read existing confirmed booking seats: %w", err)
		}
		if len(confirmed.SeatIDs) == 0 {
			// The booking is already confirmed, but it has no associated seats, which is an invalid state.
			return nil, domain.ErrBookingNotConfirmable
		}
		if err := tx.Commit(); err != nil {
			// The booking is already confirmed, but we failed to commit the transaction for some reason.
			return nil, fmt.Errorf("commit existing booking confirmation read: %w", err)
		}
		return confirmed, nil
	}
	if status != "PENDING" || !unexpired {
		// The booking is not in a state that allows confirmation.
		return nil, domain.ErrBookingNotConfirmable
	}

	var expectedSeats int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM booking_items
		WHERE booking_id = $1`, bookingID).Scan(&expectedSeats); err != nil {
		return nil, fmt.Errorf("count booking seats for confirmation: %w", err)
	}
	if expectedSeats == 0 {
		// The booking has no associated seats, which is an invalid state.
		return nil, domain.ErrBookingNotConfirmable
	}

	rows, err := tx.QueryContext(ctx, `
		UPDATE seat_claims
		SET status = 'BOOKED', expires_at = NULL
		WHERE booking_id = $1 AND status = 'HELD'
		RETURNING seat_id`, bookingID)
	if err != nil {
		// The seat claims could not be updated to booked, so we cannot confirm the booking.
		return nil, fmt.Errorf("confirm booking seats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var seatID int64
		if err := rows.Scan(&seatID); err != nil {
			return nil, fmt.Errorf("scan confirmed booking seat: %w", err)
		}
		confirmed.SeatIDs = append(confirmed.SeatIDs, seatID)
	}
	if err := rows.Err(); err != nil {
		// The seat claims could not be read after attempting to update them to booked,
		// so we cannot confirm the booking.
		return nil, fmt.Errorf("read confirmed booking seats: %w", err)
	}
	if len(confirmed.SeatIDs) != expectedSeats {
		// The number of seat claims updated to booked does not match the expected number of seats for the booking,
		// which indicates a conflict or invalid state.
		return nil, domain.ErrBookingNotConfirmable
	}
	sort.Slice(confirmed.SeatIDs, func(i, j int) bool {
		return confirmed.SeatIDs[i] < confirmed.SeatIDs[j]
	})

	err = tx.QueryRowContext(ctx, `
		UPDATE bookings
		SET status = 'CONFIRMED', confirmed_at = NOW()
		WHERE id = $1 AND status = 'PENDING' AND expires_at > NOW()
		RETURNING confirmed_at`, bookingID).Scan(&confirmed.ConfirmedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// The booking was not updated to confirmed, which indicates that it was either not pending or had expired,
		// so we cannot confirm it.
		return nil, domain.ErrBookingNotConfirmable
	}
	if err != nil {
		// The booking could not be updated to confirmed for some reason, so we cannot confirm it.
		return nil, fmt.Errorf("confirm booking: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit booking confirmation: %w", err)
	}
	return confirmed, nil
}

func (r *BookingRepository) CancelBooking(
	ctx context.Context,
	userID, bookingID int64,
) (_ *domain.CancelledBooking, returnErr error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin booking cancellation transaction: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = tx.Rollback()
		}
	}()

	cancelled := &domain.CancelledBooking{BookingID: bookingID, Status: "CANCELLED"}
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT showtime_id, status, COALESCE(hold_token, '')
		FROM bookings
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`, bookingID, userID).
		Scan(&cancelled.ShowtimeID, &status, &cancelled.HoldToken)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load booking for cancellation: %w", err)
	}
	if status != "PENDING" {
		return nil, domain.ErrBookingNotCancellable
	}

	rows, err := tx.QueryContext(ctx, `
		DELETE FROM seat_claims
		WHERE booking_id = $1 AND status = 'HELD'
		RETURNING seat_id`, bookingID)
	if err != nil {
		return nil, fmt.Errorf("release cancelled booking seats: %w", err)
	}
	defer rows.Close() // ensure rows are closed even if we return early

	for rows.Next() {
		var seatID int64
		if err := rows.Scan(&seatID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan cancelled booking seat: %w", err)
		}
		cancelled.SeatIDs = append(cancelled.SeatIDs, seatID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read cancelled booking seats: %w", err)
	}
	sort.Slice(cancelled.SeatIDs, func(i, j int) bool {
		return cancelled.SeatIDs[i] < cancelled.SeatIDs[j]
	})

	result, err := tx.ExecContext(ctx, `
		UPDATE bookings
		SET status = 'CANCELLED'
		WHERE id = $1 AND status = 'PENDING'`, bookingID)
	if err != nil {
		return nil, fmt.Errorf("cancel booking: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read cancelled booking result: %w", err)
	}
	if updated != 1 {
		return nil, domain.ErrBookingNotCancellable
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit booking cancellation: %w", err)
	}
	return cancelled, nil
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
