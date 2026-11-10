// Command migrate applies goose migrations embedded from db/migrations.
//
//	migrate up | down | status | version
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/joho/godotenv/autoload"
	"github.com/pressly/goose/v3"

	"movie_booking/db/migrations"
	"movie_booking/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "up"
	if len(args) > 0 {
		cmd = args[0]
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	ctx := context.Background()
	switch cmd {
	case "up":
		return goose.UpContext(ctx, db, ".")
	case "down":
		return goose.DownContext(ctx, db, ".")
	case "status":
		return goose.StatusContext(ctx, db, ".")
	case "version":
		return goose.VersionContext(ctx, db, ".")
	default:
		return fmt.Errorf("unknown command %q (want up|down|status|version)", cmd)
	}
}
