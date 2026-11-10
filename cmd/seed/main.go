// Command seed loads a demo catalogue: cinemas, halls with seat layouts, movies and
// upcoming showtimes. Every ID is deterministic (UUIDv5), so re-running is safe.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/joho/godotenv/autoload"

	"movie_booking/internal/adapter/postgres"
	"movie_booking/internal/config"
)

var ns = uuid.MustParse("6f1c2a52-0d1e-4c57-9a2b-7e3d5f8a9b10")

func id(name string) uuid.UUID { return uuid.NewSHA1(ns, []byte(name)) }

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

type movie struct {
	key, title, desc, cert, poster string
	minutes                        int
	genres                         []string
	imdb                           string
	tmdb                           int64
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, 5)
	if err != nil {
		return err
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Cinema + hall with a 6x10 layout, VIP back row.
	cinemaID, hallID := id("cinema:central"), id("hall:central:1")
	if _, err := tx.Exec(ctx, `
		INSERT INTO cinemas (id, name, address, city) VALUES ($1, 'Central Cinema', '1 Le Loi', 'Ho Chi Minh City')
		ON CONFLICT (id) DO NOTHING`, cinemaID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO halls (id, cinema_id, name) VALUES ($1, $2, 'Hall 1')
		ON CONFLICT (id) DO NOTHING`, hallID, cinemaID); err != nil {
		return err
	}
	for _, row := range []string{"A", "B", "C", "D", "E", "F"} {
		for n := 1; n <= 10; n++ {
			seatType, modifier := "STANDARD", int64(0)
			if row == "F" {
				seatType, modifier = "VIP", 50_000
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO seats (id, hall_id, row_label, seat_number, seat_type, price_modifier_cents)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (id) DO NOTHING`,
				id(fmt.Sprintf("seat:%s:%s%d", hallID, row, n)), hallID, row, n, seatType, modifier); err != nil {
				return err
			}
		}
	}

	movies := []movie{
		{key: "shawshank", title: "The Shawshank Redemption", desc: "Two imprisoned men bond over a number of years.", cert: "R", minutes: 142,
			genres: []string{"Drama"}, imdb: "tt0111161", tmdb: 278},
		{key: "inception", title: "Inception", desc: "A thief who steals secrets through dream-sharing is given a final job.", cert: "PG-13", minutes: 148,
			genres: []string{"Sci-Fi", "Thriller"}, imdb: "tt1375666", tmdb: 27205},
		{key: "spirited", title: "Spirited Away", desc: "A girl wanders into a world ruled by gods and witches.", cert: "PG", minutes: 125,
			genres: []string{"Animation", "Fantasy"}, imdb: "tt0245429", tmdb: 129},
	}
	for _, m := range movies {
		if _, err := tx.Exec(ctx, `
			INSERT INTO movies (id, title, description, duration_minutes, genres, certificate, imdb_id, tmdb_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING`,
			id("movie:"+m.key), m.title, m.desc, m.minutes, m.genres, m.cert, m.imdb, m.tmdb); err != nil {
			return err
		}
	}

	// One hall, so showtimes run back to back each day (the schema forbids overlaps).
	dayStart := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	showtimes := 0
	for day := 0; day < 3; day++ {
		cursor := dayStart.AddDate(0, 0, day).Add(10 * time.Hour)
		for _, m := range movies {
			startsAt := cursor
			ends := startsAt.Add(time.Duration(m.minutes+15) * time.Minute)
			cursor = ends.Add(30 * time.Minute)

			// ON CONFLICT does not cover the hall-overlap exclusion constraint, so skip existing rows explicitly.
			showID := id(fmt.Sprintf("showtime:%s:%s", m.key, startsAt.Format(time.RFC3339)))
			if _, err := tx.Exec(ctx, `
				INSERT INTO showtimes (id, movie_id, hall_id, starts_at, ends_at, base_price_cents)
				SELECT $1, $2, $3, $4, $5, 90000
				WHERE NOT EXISTS (SELECT 1 FROM showtimes WHERE id = $1)`,
				showID, id("movie:"+m.key), hallID, startsAt, ends); err != nil {
				return fmt.Errorf("showtime %s: %w", startsAt, err)
			}
			showtimes++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	slog.Info("seed complete", "cinema_id", cinemaID, "hall_id", hallID, "movies", len(movies), "showtimes", showtimes)
	return nil
}
