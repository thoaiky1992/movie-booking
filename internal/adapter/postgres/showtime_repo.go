package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

type ShowtimeRepo struct{ pool *pgxpool.Pool }

func NewShowtimeRepo(pool *pgxpool.Pool) *ShowtimeRepo { return &ShowtimeRepo{pool: pool} }

var _ usecase.ShowtimeRepository = (*ShowtimeRepo)(nil)

// List filters on (movie_id, starts_at) and (hall_id, starts_at) indexes.
func (r *ShowtimeRepo) List(ctx context.Context, f usecase.ShowtimeFilter) ([]domain.ShowtimeView, int, error) {
	const filter = `
		WHERE ($1::uuid IS NULL OR s.movie_id = $1)
		  AND ($2::uuid IS NULL OR h.cinema_id = $2)
		  AND ($3::timestamptz IS NULL OR s.starts_at >= $3)
		  AND ($4::timestamptz IS NULL OR s.starts_at < $4)`
	const from = `
		FROM showtimes s
		JOIN movies m ON m.id = s.movie_id
		JOIN halls h  ON h.id = s.hall_id
		JOIN cinemas c ON c.id = h.cinema_id`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) `+from+filter,
		f.MovieID, f.CinemaID, f.From, f.To).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count showtimes: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.movie_id, s.hall_id, s.starts_at, s.ends_at, s.base_price_cents,
		       m.title, c.id, c.name, h.name
		`+from+filter+`
		ORDER BY s.starts_at, s.id
		LIMIT $5 OFFSET $6`, f.MovieID, f.CinemaID, f.From, f.To, f.Page.Limit(), f.Page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list showtimes: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ShowtimeView, 0, f.Page.Size)
	for rows.Next() {
		var v domain.ShowtimeView
		if err := rows.Scan(&v.ID, &v.MovieID, &v.HallID, &v.StartsAt, &v.EndsAt, &v.BasePriceCents,
			&v.MovieTitle, &v.CinemaID, &v.CinemaName, &v.HallName); err != nil {
			return nil, 0, fmt.Errorf("scan showtime: %w", err)
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (r *ShowtimeRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Showtime, error) {
	var s domain.Showtime
	err := r.pool.QueryRow(ctx, `
		SELECT id, movie_id, hall_id, starts_at, ends_at, base_price_cents
		FROM showtimes WHERE id = $1`, id).
		Scan(&s.ID, &s.MovieID, &s.HallID, &s.StartsAt, &s.EndsAt, &s.BasePriceCents)
	if err != nil {
		return nil, notFoundOr(err, "showtime")
	}
	return &s, nil
}

// SeatMap joins the hall layout to live booking_seats. A HELD row only counts
// while its booking's deadline is in the future, so expired holds read as free
// even before the sweeper has run.
func (r *ShowtimeRepo) SeatMap(ctx context.Context, showtimeID uuid.UUID, now time.Time) ([]domain.SeatStatus, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.hall_id, s.row_label, s.seat_number, s.seat_type, s.price_modifier_cents,
		       CASE
		         WHEN bs.status = 'CONFIRMED' THEN 'SOLD'
		         WHEN bs.status = 'HELD'      THEN 'HELD'
		         ELSE 'AVAILABLE'
		       END AS status,
		       st.base_price_cents + s.price_modifier_cents AS price_cents
		FROM showtimes st
		JOIN seats s ON s.hall_id = st.hall_id
		LEFT JOIN booking_seats bs
		       ON bs.showtime_id = st.id
		      AND bs.seat_id = s.id
		      AND bs.status IN ('HELD', 'CONFIRMED')
		LEFT JOIN bookings b
		       ON b.id = bs.booking_id
		WHERE st.id = $1
		  AND (bs.status IS NULL OR bs.status = 'CONFIRMED' OR b.expires_at > $2)
		ORDER BY s.row_label, s.seat_number`, showtimeID, now)
	if err != nil {
		return nil, fmt.Errorf("query seat map: %w", err)
	}
	defer rows.Close()

	var out []domain.SeatStatus
	for rows.Next() {
		var ss domain.SeatStatus
		if err := rows.Scan(&ss.ID, &ss.HallID, &ss.RowLabel, &ss.SeatNumber, &ss.SeatType,
			&ss.PriceModifierCents, &ss.Status, &ss.PriceCents); err != nil {
			return nil, fmt.Errorf("scan seat status: %w", err)
		}
		out = append(out, ss)
	}
	return out, rows.Err()
}
