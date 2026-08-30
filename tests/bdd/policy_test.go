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

// TestCanViewSightingMedia exercises the storage-policy helper from
// db/migrations/00007_recordings.sql directly.
func TestCanViewSightingMedia(t *testing.T) {
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

	const (
		ownerAuth    = "11111111-1111-1111-1111-111111111111"
		viewerAuth   = "22222222-2222-2222-2222-222222222222"
		strangerAuth = "33333333-3333-3333-3333-333333333333"
	)
	ownerID := "usr_owner" + strings.Repeat("0", 21)
	viewerID := "usr_viewer" + strings.Repeat("0", 20)
	strangerID := "usr_strangr" + strings.Repeat("0", 19)
	groupID := "grp_group" + strings.Repeat("0", 21)
	sghLive := "sgh_live" + strings.Repeat("0", 22)
	sghDead := "sgh_dead" + strings.Repeat("0", 22)

	livePhoto := ownerAuth + "/" + sghLive + "/a.jpg"
	liveRec := ownerAuth + "/" + sghLive + "/a.webm"
	unattached := ownerAuth + "/" + sghLive + "/orphan.jpg" // same prefix, never attached
	deadPhoto := ownerAuth + "/" + sghDead + "/d.jpg"

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}

	mustExec(
		`INSERT INTO users (id, auth_id, email) VALUES ($1,$2,$3),($4,$5,$6),($7,$8,$9)`,
		ownerID, ownerAuth, "owner@test.co",
		viewerID, viewerAuth, "viewer@test.co",
		strangerID, strangerAuth, "stranger@test.co",
	)
	// Owner and viewer share a group; stranger is in no group.
	mustExec(`INSERT INTO groups (id, name, owner_user_id, join_code) VALUES ($1,'Walkers',$2,'JOINCODE1')`, groupID, ownerID)
	mustExec(`INSERT INTO group_members (group_id, user_id) VALUES ($1,$2),($1,$3)`, groupID, ownerID, viewerID)
	mustExec(
		`INSERT INTO sightings (id, user_id, observed_at, client_updated_at, photo_paths, recording_paths)
		 VALUES ($1,$2, now(), now(), $3::text[], $4::text[])`,
		sghLive, ownerID, "{"+livePhoto+"}", "{"+liveRec+"}",
	)
	mustExec(
		`INSERT INTO sightings (id, user_id, observed_at, client_updated_at, photo_paths, deleted_at)
		 VALUES ($1,$2, now(), now(), $3::text[], now())`,
		sghDead, ownerID, "{"+deadPhoto+"}",
	)

	cases := []struct {
		name   string
		viewer string
		object string
		want   bool
	}{
		{"co-member sees attached photo", viewerAuth, livePhoto, true},
		{"co-member sees attached recording", viewerAuth, liveRec, true},
		{"stranger sees nothing", strangerAuth, livePhoto, false},
		{"co-member denied unattached object under same prefix", viewerAuth, unattached, false},
		{"co-member denied media on a deleted sighting", viewerAuth, deadPhoto, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got bool
			if err := db.GetContext(ctx, &got,
				`SELECT can_view_sighting_media($1::uuid, $2)`, tc.viewer, tc.object,
			); err != nil {
				t.Fatalf("query: %v", err)
			}
			if got != tc.want {
				t.Errorf("can_view_sighting_media(%s, %s) = %v, want %v", tc.viewer, tc.object, got, tc.want)
			}
		})
	}
}
