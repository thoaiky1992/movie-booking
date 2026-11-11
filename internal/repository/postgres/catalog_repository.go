package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"movie-booking/internal/domain"
)

type CatalogRepository struct {
	db *sql.DB
}

func NewCatalogRepository(db *sql.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

func (r *CatalogRepository) ListCinemas(ctx context.Context) ([]domain.Cinema, error) {
	const query = `
		SELECT id, name, created_at
		FROM cinemas
		ORDER BY name, id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query cinemas: %w", err)
	}
	defer rows.Close()

	cinemas := make([]domain.Cinema, 0)
	for rows.Next() {
		var cinema domain.Cinema
		if err := rows.Scan(
			&cinema.ID,
			&cinema.Name,
			&cinema.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cinema: %w", err)
		}
		cinemas = append(cinemas, cinema)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cinemas: %w", err)
	}

	return cinemas, nil
}

func (r *CatalogRepository) ListMoviesByCinema(ctx context.Context, cinemaID int64) ([]domain.Movie, error) {
	const query = `
		SELECT DISTINCT
			m.id,
			m.title,
			m.duration_minutes,
			m.created_at
		FROM movies m
		JOIN showtimes st ON st.movie_id = m.id
		JOIN rooms r ON r.id = st.room_id
		WHERE r.cinema_id = $1
		ORDER BY m.title, m.id`

	rows, err := r.db.QueryContext(ctx, query, cinemaID)
	if err != nil {
		return nil, fmt.Errorf("query movies by cinema: %w", err)
	}
	defer rows.Close()

	movies := make([]domain.Movie, 0)
	for rows.Next() {
		var movie domain.Movie
		if err := rows.Scan(
			&movie.ID,
			&movie.Title,
			&movie.DurationMinutes,
			&movie.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan movie: %w", err)
		}
		movies = append(movies, movie)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate movies: %w", err)
	}

	return movies, nil
}

func (r *CatalogRepository) GetMovieDetail(ctx context.Context, movieID int64) (*domain.MovieDetail, error) {
	const movieQuery = `
		SELECT
			m.id,
			m.title,
			m.duration_minutes,
			m.created_at,
			md.synopsis,
			md.director,
			md.cast_members,
			md.genres,
			md.poster_url,
			md.trailer_url,
			TO_CHAR(md.release_date, 'YYYY-MM-DD'),
			md.age_rating,
			md.original_language,
			md.updated_at
		FROM movies m
		LEFT JOIN movie_details md ON md.movie_id = m.id
		WHERE m.id = $1`

	movie := &domain.MovieDetail{}
	err := r.db.QueryRowContext(ctx, movieQuery, movieID).Scan(
		&movie.ID,
		&movie.Title,
		&movie.DurationMinutes,
		&movie.CreatedAt,
		&movie.Synopsis,
		&movie.Director,
		&movie.CastMembers,
		&movie.Genres,
		&movie.PosterURL,
		&movie.TrailerURL,
		&movie.ReleaseDate,
		&movie.AgeRating,
		&movie.OriginalLanguage,
		&movie.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrMovieNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query movie detail: %w", err)
	}

	const ratingsQuery = `
		SELECT
			id,
			source,
			rating_score,
			max_score,
			vote_count,
			external_url,
			last_synced_at
		FROM movie_external_ratings
		WHERE movie_id = $1
		ORDER BY source, id`

	rows, err := r.db.QueryContext(ctx, ratingsQuery, movieID)
	if err != nil {
		return nil, fmt.Errorf("query movie external ratings: %w", err)
	}
	defer rows.Close()

	movie.ExternalRatings = make([]domain.MovieExternalRating, 0)
	for rows.Next() {
		var rating domain.MovieExternalRating
		if err := rows.Scan(
			&rating.ID,
			&rating.Source,
			&rating.RatingScore,
			&rating.MaxScore,
			&rating.VoteCount,
			&rating.ExternalURL,
			&rating.LastSyncedAt,
		); err != nil {
			return nil, fmt.Errorf("scan movie external rating: %w", err)
		}
		movie.ExternalRatings = append(movie.ExternalRatings, rating)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate movie external ratings: %w", err)
	}

	return movie, nil
}

func (r *CatalogRepository) ListShowtimesByCinemaAndMovie(
	ctx context.Context,
	cinemaID, movieID int64,
) ([]domain.Showtime, error) {
	const query = `
		SELECT
			s.id,
			s.starts_at,
			s.ends_at,
			m.id,
			m.title,
			m.duration_minutes,
			c.id,
			c.name,
			r.id,
			r.name
		FROM showtimes s
		JOIN movies m ON m.id = s.movie_id
		JOIN rooms r ON r.id = s.room_id
		JOIN cinemas c ON c.id = r.cinema_id
		WHERE c.id = $1 AND m.id = $2
		ORDER BY s.starts_at, r.name, s.id`

	rows, err := r.db.QueryContext(ctx, query, cinemaID, movieID)
	if err != nil {
		return nil, fmt.Errorf("query showtimes by cinema and movie: %w", err)
	}
	defer rows.Close()

	showtimes := make([]domain.Showtime, 0)
	for rows.Next() {
		var showtime domain.Showtime
		if err := rows.Scan(
			&showtime.ID,
			&showtime.StartsAt,
			&showtime.EndsAt,
			&showtime.Movie.ID,
			&showtime.Movie.Title,
			&showtime.Movie.DurationMinutes,
			&showtime.Cinema.ID,
			&showtime.Cinema.Name,
			&showtime.Room.ID,
			&showtime.Room.Name,
		); err != nil {
			return nil, fmt.Errorf("scan showtime: %w", err)
		}
		showtimes = append(showtimes, showtime)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate showtimes: %w", err)
	}

	return showtimes, nil
}

func (r *CatalogRepository) GetSeatMap(ctx context.Context, showtimeID int64) (*domain.SeatMap, error) {
	const showtimeQuery = `
		SELECT
			s.id,
			s.starts_at,
			s.ends_at,
			m.id,
			m.title,
			m.duration_minutes,
			c.id,
			c.name,
			r.id,
			r.name
		FROM showtimes s
		JOIN movies m ON m.id = s.movie_id
		JOIN rooms r ON r.id = s.room_id
		JOIN cinemas c ON c.id = r.cinema_id
		WHERE s.id = $1`

	seatMap := &domain.SeatMap{}
	err := r.db.QueryRowContext(ctx, showtimeQuery, showtimeID).Scan(
		&seatMap.ShowtimeID,
		&seatMap.StartsAt,
		&seatMap.EndsAt,
		&seatMap.Movie.ID,
		&seatMap.Movie.Title,
		&seatMap.Movie.DurationMinutes,
		&seatMap.Cinema.ID,
		&seatMap.Cinema.Name,
		&seatMap.Room.ID,
		&seatMap.Room.Name,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrShowtimeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query showtime: %w", err)
	}

	const seatsQuery = `
		SELECT
			se.id,
			se.row_label,
			se.seat_number,
			CASE
				WHEN sc.status = 'BOOKED' THEN 'BOOKED'
				WHEN sc.status = 'HELD' AND sc.expires_at > NOW() THEN 'HELD'
				ELSE 'AVAILABLE'
			END AS seat_status
		FROM seats se
		JOIN showtimes st ON st.room_id = se.room_id
		LEFT JOIN seat_claims sc
			ON sc.showtime_id = st.id
			AND sc.seat_id = se.id
		WHERE st.id = $1
		ORDER BY se.row_label, se.seat_number`

	rows, err := r.db.QueryContext(ctx, seatsQuery, showtimeID)
	if err != nil {
		return nil, fmt.Errorf("query seats: %w", err)
	}
	defer rows.Close()

	seatMap.Rows = make([]domain.SeatRow, 0)
	var currentRow *domain.SeatRow
	for rows.Next() {
		var (
			seat     domain.Seat
			rowLabel string
			status   string
		)
		if err := rows.Scan(&seat.ID, &rowLabel, &seat.Number, &status); err != nil {
			return nil, fmt.Errorf("scan seat: %w", err)
		}
		seat.Status = domain.SeatStatus(status)

		if currentRow == nil || currentRow.Label != rowLabel {
			seatMap.Rows = append(seatMap.Rows, domain.SeatRow{
				Label: rowLabel,
				Seats: make([]domain.Seat, 0),
			})
			currentRow = &seatMap.Rows[len(seatMap.Rows)-1]
		}
		currentRow.Seats = append(currentRow.Seats, seat)
		addSeatToSummary(&seatMap.Summary, seat.Status)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate seats: %w", err)
	}

	return seatMap, nil
}

func addSeatToSummary(summary *domain.SeatSummary, status domain.SeatStatus) {
	summary.Total++
	switch status {
	case domain.SeatHeld:
		summary.Held++
	case domain.SeatBooked:
		summary.Booked++
	default:
		summary.Available++
	}
}
