package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

type MovieRepo struct{ pool *pgxpool.Pool }

func NewMovieRepo(pool *pgxpool.Pool) *MovieRepo { return &MovieRepo{pool: pool} }

var _ usecase.MovieRepository = (*MovieRepo)(nil)

const movieColumns = `id, title, description, duration_minutes, genres, certificate, release_date, poster_url, imdb_id, tmdb_id`

// List uses the pg_trgm index for title search and the GIN index for genre containment.
func (r *MovieRepo) List(ctx context.Context, f usecase.MovieFilter) ([]domain.Movie, int, error) {
	const filter = `
		WHERE ($1 = '' OR title ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR genres @> ARRAY[$2]::text[])`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM movies`+filter, f.Query, f.Genre).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count movies: %w", err)
	}

	rows, err := r.pool.Query(ctx, `SELECT `+movieColumns+` FROM movies`+filter+`
		ORDER BY release_date DESC NULLS LAST, title, id
		LIMIT $3 OFFSET $4`, f.Query, f.Genre, f.Page.Limit(), f.Page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list movies: %w", err)
	}
	defer rows.Close()

	movies := make([]domain.Movie, 0, f.Page.Size)
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan movie: %w", err)
		}
		movies = append(movies, m)
	}
	return movies, total, rows.Err()
}

func (r *MovieRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Movie, error) {
	m, err := scanMovie(r.pool.QueryRow(ctx, `SELECT `+movieColumns+` FROM movies WHERE id = $1`, id))
	if err != nil {
		return nil, notFoundOr(err, "movie")
	}
	return &m, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMovie(row rowScanner) (domain.Movie, error) {
	var m domain.Movie
	var genres []string
	err := row.Scan(&m.ID, &m.Title, &m.Description, &m.DurationMinutes, &genres,
		&m.Certificate, &m.ReleaseDate, &m.PosterURL, &m.IMDbID, &m.TMDbID)
	m.Genres = genres
	return m, err
}

// notFoundOr maps pgx.ErrNoRows to domain.ErrNotFound and passes nil through.
func notFoundOr(err error, what string) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return fmt.Errorf("%s: %w", what, domain.ErrNotFound)
	}
	return fmt.Errorf("load %s: %w", what, err)
}
