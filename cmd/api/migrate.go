package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"

	"github.com/ivanw98/birb/db/migrations"
)

// runMigrate applies every embedded migration to DATABASE_URL, read straight from the environment
// rather than via config.Load(), which also requires the auth settings a schema change has no use for.
func runMigrate(ctx context.Context, log *slog.Logger) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	// "." is the embed FS root: embed.go sits in db/migrations beside the SQL.
	if err := goose.UpContext(ctx, db.DB, "."); err != nil {
		return err
	}
	log.Info("migrations applied")
	return nil
}
