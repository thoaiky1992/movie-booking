package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

// CinemaRepo covers cinemas, halls and seat layouts.
type CinemaRepo struct{ pool *pgxpool.Pool }

func NewCinemaRepo(pool *pgxpool.Pool) *CinemaRepo { return &CinemaRepo{pool: pool} }

var (
	_ usecase.CinemaRepository = (*CinemaRepo)(nil)
	_ usecase.SeatRepository   = (*CinemaRepo)(nil)
)

func (r *CinemaRepo) List(ctx context.Context, p usecase.Page) ([]domain.Cinema, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM cinemas`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cinemas: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, address, city FROM cinemas
		ORDER BY city, name, id LIMIT $1 OFFSET $2`, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list cinemas: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Cinema, 0, p.Size)
	for rows.Next() {
		var c domain.Cinema
		if err := rows.Scan(&c.ID, &c.Name, &c.Address, &c.City); err != nil {
			return nil, 0, fmt.Errorf("scan cinema: %w", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *CinemaRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cinema, error) {
	var c domain.Cinema
	err := r.pool.QueryRow(ctx, `SELECT id, name, address, city FROM cinemas WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Address, &c.City)
	if err != nil {
		return nil, notFoundOr(err, "cinema")
	}
	return &c, nil
}

func (r *CinemaRepo) ListHalls(ctx context.Context, cinemaID uuid.UUID) ([]domain.Hall, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, cinema_id, name FROM halls WHERE cinema_id = $1 ORDER BY name`, cinemaID)
	if err != nil {
		return nil, fmt.Errorf("list halls: %w", err)
	}
	defer rows.Close()

	var out []domain.Hall
	for rows.Next() {
		var h domain.Hall
		if err := rows.Scan(&h.ID, &h.CinemaID, &h.Name); err != nil {
			return nil, fmt.Errorf("scan hall: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetByIDs returns only seats that belong to hallID, so a seat from another
// hall can never be booked through a showtime it does not belong to.
func (r *CinemaRepo) GetByIDs(ctx context.Context, hallID uuid.UUID, ids []uuid.UUID) ([]domain.Seat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, hall_id, row_label, seat_number, seat_type, price_modifier_cents
		FROM seats WHERE hall_id = $1 AND id = ANY($2)`, hallID, ids)
	if err != nil {
		return nil, fmt.Errorf("load seats: %w", err)
	}
	defer rows.Close()

	var out []domain.Seat
	for rows.Next() {
		var s domain.Seat
		if err := rows.Scan(&s.ID, &s.HallID, &s.RowLabel, &s.SeatNumber, &s.SeatType, &s.PriceModifierCents); err != nil {
			return nil, fmt.Errorf("scan seat: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
