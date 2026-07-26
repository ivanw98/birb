package store

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/ivanw98/birb/internal/models"
)

// BirdStore is the Postgres-backed BirdRepository.
type BirdStore struct {
	db      *sqlx.DB
	columns []string
}

// NewBirdStore initializes the store and caches the database columns from models.Bird.
func NewBirdStore(db *sqlx.DB) *BirdStore {
	return &BirdStore{
		db:      db,
		columns: getColumns(models.Bird{}),
	}
}

var _ BirdRepository = (*BirdStore)(nil)

const birdsTable = "birds"

// List returns all birds in taxonomic (list) order.
func (s *BirdStore) List(ctx context.Context) ([]models.Bird, error) {
	query, args, err := builder.
		Select(s.columns...).
		From(birdsTable).
		OrderBy("taxonomic_order ASC NULLS LAST", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build birds list: %w", err)
	}

	birds := []models.Bird{}
	if err := s.db.SelectContext(ctx, &birds, query, args...); err != nil {
		return nil, err
	}
	return birds, nil
}

// ExistingIDs returns the subset of ids that exist in the birds table, or an empty set without a query when ids is empty.
func (s *BirdStore) ExistingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	out := make(map[string]struct{}, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	query, args, err := builder.
		Select("id").
		From(birdsTable).
		Where(sq.Eq{"id": ids}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build birds existing-ids: %w", err)
	}

	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}
