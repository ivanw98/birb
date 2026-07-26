package store

import (
	"context"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/ivanw98/birb/internal/models"
)

// UserStore is the Postgres-backed UserRepository.
type UserStore struct {
	db      *sqlx.DB
	columns []string
}

// NewUserStore initializes the store and caches the database columns from the models.User struct.
func NewUserStore(db *sqlx.DB) *UserStore {
	return &UserStore{
		db:      db,
		columns: getColumns(models.User{}),
	}
}

var _ UserRepository = (*UserStore)(nil)

const (
	usersTable = "users"
)

// Upsert provisions or refreshes a user keyed by auth_id, preserving the existing id and refreshing email/display_name on conflict.
func (s *UserStore) Upsert(ctx context.Context, u models.User) (models.User, error) {
	returningCols := strings.Join(s.columns, ", ")

	// squirrel has no native upsert; the ON CONFLICT ... RETURNING clause is expressed as a raw Suffix.
	query, args, err := builder.
		Insert(usersTable).
		Columns("id", "auth_id", "email", "display_name").
		Values(u.ID, u.AuthID, u.Email, u.DisplayName).
		Suffix(`ON CONFLICT (auth_id) DO UPDATE
SET email = EXCLUDED.email,
    display_name = EXCLUDED.display_name,
    updated_at = now()
RETURNING ` + returningCols).
		ToSql()
	if err != nil {
		return models.User{}, fmt.Errorf("build users upsert: %w", err)
	}

	var out models.User
	err = s.db.QueryRowxContext(ctx, query, args...).StructScan(&out)
	return out, err
}

// GetByAuthID returns the user for a Supabase auth id, or sql.ErrNoRows.
func (s *UserStore) GetByAuthID(ctx context.Context, authID string) (models.User, error) {
	query, args, err := builder.
		Select(s.columns...).
		From(usersTable).
		Where(sq.Eq{"auth_id": authID}).
		ToSql()
	if err != nil {
		return models.User{}, fmt.Errorf("build users get: %w", err)
	}

	var out models.User
	err = s.db.QueryRowxContext(ctx, query, args...).StructScan(&out)
	return out, err
}
