// Package store is the repository layer: interfaces plus their sqlx + squirrel implementations over Postgres.
package store

import (
	"context"
	"reflect"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/ivanw98/birb/internal/models"
)

// builder is the shared squirrel statement builder using Postgres ($1, $2, …) placeholders.
var builder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// DB is the subset of *sqlx.DB the stores use.
type DB interface {
	sqlx.ExtContext
}

// UserRepository provisions and reads user records.
type UserRepository interface {
	// Upsert inserts or refreshes a user by auth_id and returns the canonical row; idempotent, safe to call on every request.
	Upsert(ctx context.Context, u models.User) (models.User, error)
	// GetByAuthID returns the user for a Supabase auth id, or ErrNoRows.
	GetByAuthID(ctx context.Context, authID string) (models.User, error)
}

// BirdRepository reads the static species reference list.
type BirdRepository interface {
	// List returns all birds in taxonomic (list) order.
	List(ctx context.Context) ([]models.Bird, error)
	// ExistingIDs returns the subset of the given ids that exist, for bulk-validating sighting bird references.
	ExistingIDs(ctx context.Context, ids []string) (map[string]struct{}, error)
}

// UpsertOutcome reports how a batch upsert resolved for one sighting.
type UpsertOutcome struct {
	Status models.BatchItemStatus // created | updated | stale
	// Conflict is true when the id already exists but is owned by another user;
	// the service maps this to an invalid/id_conflict item.
	Conflict bool
}

// SightingRepository is the data access for sightings.
type SightingRepository interface {
	// Upsert applies one batch item as insert-or-update-if-newer scoped to the owner, setting capture fields only on insert (see UpsertOutcome).
	Upsert(ctx context.Context, s models.Sighting) (UpsertOutcome, error)
	// ListByUser returns up to limit rows for the user, newest first, resuming
	// after cursor when non-nil; tombstones are excluded unless includeDeleted.
	ListByUser(ctx context.Context, userID string, cursor *models.Cursor, limit int, includeDeleted bool) ([]models.Sighting, error)
	// GetForUser returns the caller's sighting, or found=false if it does not
	// exist or belongs to someone else.
	GetForUser(ctx context.Context, id, userID string) (row *models.Sighting, found bool, err error)
	// UpdateContent replaces the mutable content fields when the stored client_updated_at is not newer than upd.ClientUpdatedAt, reporting applied=false when the guard failed or the row is not the caller's.
	UpdateContent(ctx context.Context, id, userID string, upd models.SightingUpdate) (row *models.Sighting, applied bool, err error)
}

// Store bundles the concrete repositories over a single database handle.
type Store struct {
	Users     *UserStore
	Birds     *BirdStore
	Sightings *SightingStore
}

// New builds the repository set over db.
func New(db *sqlx.DB) *Store {
	return &Store{
		Users:     NewUserStore(db),
		Birds:     NewBirdStore(db),
		Sightings: NewSightingStore(db),
	}
}

// getColumns extracts the "db" tag values from a struct to use as SQL columns.
func getColumns(s any) []string {
	typ := reflect.TypeOf(s)

	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return []string{}
	}

	var columns []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		dbTag := field.Tag.Get("db")

		if dbTag != "" && dbTag != "-" {
			colName := strings.Split(dbTag, ",")[0]
			columns = append(columns, colName)
		}
	}

	return columns
}
