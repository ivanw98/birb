//go:build bdd

package bdd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

// TestSightingMediaPathByteCaps exercises the DB-level aggregate CHECK
// constraints on photo_paths and recording_paths directly.
func TestSightingMediaPathByteCaps(t *testing.T) {
	dsn := os.Getenv("BIRB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("BIRB_TEST_DATABASE_URL not set; see tests/bdd/README.md")
	}

	ctx := context.Background()
	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db.DB, filepath.Join("..", "..", "db", "migrations")); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`TRUNCATE public.group_members, public.groups, public.sightings, public.users RESTART IDENTITY CASCADE`,
	); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// Every sightings/users id is checked at the DB level against
	// ^(usr|sgh)_[a-z0-9]{26}$ (db/migrations/00001_init.sql), so ids here
	// must pad to exactly 26 lowercase-alphanumeric chars after the prefix.
	idFor := func(prefix, short string) string {
		if len(short) > 26 {
			t.Fatalf("test id fragment %q exceeds 26 chars", short)
		}
		return prefix + "_" + short + strings.Repeat("0", 26-len(short))
	}

	userID := idFor("usr", "capsowner")
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, auth_id, email) VALUES ($1, $2, $3)`,
		userID, "44444444-4444-4444-4444-444444444444", "capsowner@test.co",
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	insertSighting := func(id string, photoPaths, recordingPaths []string) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO sightings (id, user_id, observed_at, client_updated_at, photo_paths, recording_paths)
			 VALUES ($1, $2, now(), now(), $3::text[], $4::text[])`,
			id, userID, "{"+strings.Join(photoPaths, ",")+"}", "{"+strings.Join(recordingPaths, ",")+"}",
		)
		return err
	}

	repeatPaths := func(n, length int) []string {
		paths := make([]string, n)
		for i := range paths {
			paths[i] = strings.Repeat("a", length)
		}
		return paths
	}

	t.Run("10 photo paths at exactly 512 bytes each is accepted (10*512+9=5129)", func(t *testing.T) {
		if err := insertSighting(idFor("sgh", "capsphotookay"), repeatPaths(10, 512), nil); err != nil {
			t.Fatalf("expected acceptance at the boundary, got: %v", err)
		}
	})

	t.Run("10 photo paths one byte over is rejected", func(t *testing.T) {
		paths := repeatPaths(10, 512)
		paths[0] = strings.Repeat("a", 513)
		if err := insertSighting(idFor("sgh", "capsphotobad"), paths, nil); err == nil {
			t.Fatal("expected a constraint violation, got none")
		}
	})

	t.Run("5 recording paths at exactly 512 bytes each is accepted (5*512+4=2564)", func(t *testing.T) {
		if err := insertSighting(idFor("sgh", "capsrecokay"), nil, repeatPaths(5, 512)); err != nil {
			t.Fatalf("expected acceptance at the boundary, got: %v", err)
		}
	})

	t.Run("5 recording paths one byte over is rejected", func(t *testing.T) {
		paths := repeatPaths(5, 512)
		paths[0] = strings.Repeat("a", 513)
		if err := insertSighting(idFor("sgh", "capsrecbad"), nil, paths); err == nil {
			t.Fatal("expected a constraint violation, got none")
		}
	})
}
