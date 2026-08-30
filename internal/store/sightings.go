package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/ivanw98/birb/internal/models"
)

const sightingsTable = "sightings"

// SightingStore is the Postgres-backed SightingRepository.
type SightingStore struct {
	db            *sqlx.DB
	selectColumns []string
	returningCols string
}

var _ SightingRepository = (*SightingStore)(nil)

// NewSightingStore initializes the store and caches the database columns.
func NewSightingStore(db *sqlx.DB) *SightingStore {
	rawCols := getColumns(sightingRow{})
	selectCols := make([]string, len(rawCols))

	// photo_paths and recording_paths need an explicit ::text cast (see sightingRow).
	arrayCols := map[string]bool{"photo_paths": true, "recording_paths": true}
	for i, col := range rawCols {
		if arrayCols[col] {
			selectCols[i] = col + "::text AS " + col
		} else {
			selectCols[i] = col
		}
	}

	return &SightingStore{
		db:            db,
		selectColumns: selectCols,
		returningCols: strings.Join(selectCols, ", "),
	}
}

// sightingRow scans a sightings row, reading photo_paths/recording_paths via a ::text cast into StringArray (see arrays.go).
type sightingRow struct {
	ID                      string      `db:"id"`
	UserID                  string      `db:"user_id"`
	ObservedAt              time.Time   `db:"observed_at"`
	ObservedAtOffsetMinutes int32       `db:"observed_at_offset_minutes"`
	ClientUpdatedAt         time.Time   `db:"client_updated_at"`
	CreatedAt               time.Time   `db:"created_at"`
	UpdatedAt               time.Time   `db:"updated_at"`
	BirdID                  *string     `db:"bird_id"`
	QuickNote               *string     `db:"quick_note"`
	Notes                   *string     `db:"notes"`
	Latitude                *float64    `db:"latitude"`
	Longitude               *float64    `db:"longitude"`
	AccuracyM               *float64    `db:"accuracy_m"`
	PhotoPaths              StringArray `db:"photo_paths"`
	RecordingPaths          StringArray `db:"recording_paths"`
	DeletedAt               *time.Time  `db:"deleted_at"`
}

func (r sightingRow) toModel() models.Sighting {
	paths := []string(r.PhotoPaths)
	if paths == nil {
		paths = []string{}
	}
	recordingPaths := []string(r.RecordingPaths)
	if recordingPaths == nil {
		recordingPaths = []string{}
	}
	return models.Sighting{
		ID:                      r.ID,
		UserID:                  r.UserID,
		ObservedAt:              r.ObservedAt.UTC(),
		ObservedAtOffsetMinutes: r.ObservedAtOffsetMinutes,
		ClientUpdatedAt:         r.ClientUpdatedAt.UTC(),
		CreatedAt:               r.CreatedAt.UTC(),
		UpdatedAt:               r.UpdatedAt.UTC(),
		BirdID:                  r.BirdID,
		QuickNote:               r.QuickNote,
		Notes:                   r.Notes,
		Latitude:                r.Latitude,
		Longitude:               r.Longitude,
		AccuracyM:               r.AccuracyM,
		PhotoPaths:              paths,
		RecordingPaths:          recordingPaths,
		Deleted:                 r.DeletedAt != nil,
	}
}

// Upsert applies one batch item.
func (s *SightingStore) Upsert(ctx context.Context, in models.Sighting) (UpsertOutcome, error) {
	// A tombstone item stamps deleted_at; COALESCE below keeps the original
	// delete time when a delete is replayed.
	var deletedAt *time.Time
	if in.Deleted {
		now := time.Now().UTC()
		deletedAt = &now
	}

	// Insert columns are hardcoded (not derived via getColumns) since Upsert intentionally writes only a subset of fields, excluding photo_paths, recording_paths and timestamps.
	query, args, err := builder.
		Insert(sightingsTable).
		SetMap(map[string]interface{}{
			"id":                         in.ID,
			"user_id":                    in.UserID,
			"observed_at":                in.ObservedAt,
			"observed_at_offset_minutes": in.ObservedAtOffsetMinutes,
			"client_updated_at":          in.ClientUpdatedAt,
			"bird_id":                    in.BirdID,
			"quick_note":                 in.QuickNote,
			"notes":                      in.Notes,
			"latitude":                   in.Latitude,
			"longitude":                  in.Longitude,
			"accuracy_m":                 in.AccuracyM,
			"deleted_at":                 deletedAt,
		}).
		Suffix(`ON CONFLICT (id) DO UPDATE
			SET bird_id = EXCLUDED.bird_id,
			    quick_note = EXCLUDED.quick_note,
			    notes = EXCLUDED.notes,
			    deleted_at = CASE WHEN EXCLUDED.deleted_at IS NOT NULL
			                      THEN COALESCE(sightings.deleted_at, EXCLUDED.deleted_at)
			                      ELSE NULL END,
			    client_updated_at = EXCLUDED.client_updated_at,
			    updated_at = now()
			WHERE sightings.user_id = EXCLUDED.user_id
			  AND sightings.client_updated_at < EXCLUDED.client_updated_at
			RETURNING (xmax = 0) AS inserted`).
		ToSql()
	if err != nil {
		return UpsertOutcome{}, fmt.Errorf("build sightings upsert: %w", err)
	}

	var inserted bool
	err = s.db.QueryRowxContext(ctx, query, args...).Scan(&inserted)

	if err == nil {
		if inserted {
			return UpsertOutcome{Status: models.StatusCreated}, nil
		}
		return UpsertOutcome{Status: models.StatusUpdated}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return UpsertOutcome{}, err
	}

	ownerQuery, ownerArgs, err := builder.
		Select("user_id").
		From(sightingsTable).
		Where(sq.Eq{"id": in.ID}).
		ToSql()
	if err != nil {
		return UpsertOutcome{}, fmt.Errorf("build sightings owner lookup: %w", err)
	}
	var ownerID string
	err = s.db.QueryRowxContext(ctx, ownerQuery, ownerArgs...).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return UpsertOutcome{Status: models.StatusStale}, nil
	}
	if err != nil {
		return UpsertOutcome{}, err
	}
	if ownerID != in.UserID {
		return UpsertOutcome{Conflict: true}, nil
	}
	return UpsertOutcome{Status: models.StatusStale}, nil
}

// ListByUser returns up to limit rows for the user.
func (s *SightingStore) ListByUser(ctx context.Context, userID string, cursor *models.Cursor, limit int, includeDeleted bool) ([]models.Sighting, error) {
	q := builder.Select(s.selectColumns...).
		From(sightingsTable).
		Where(sq.Eq{"user_id": userID})

	if !includeDeleted {
		q = q.Where(sq.Eq{"deleted_at": nil})
	}

	if cursor != nil {
		q = q.Where("(observed_at, id) < (?, ?)", cursor.ObservedAt, cursor.ID)
	}

	q = q.OrderBy("observed_at DESC", "id DESC").Limit(uint64(limit))

	sqlStr, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	var rows []sightingRow
	if err := s.db.SelectContext(ctx, &rows, sqlStr, args...); err != nil {
		return nil, err
	}

	out := make([]models.Sighting, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

// GetForUser returns the caller's sighting.
func (s *SightingStore) GetForUser(ctx context.Context, id, userID string) (*models.Sighting, bool, error) {
	query, args, err := builder.
		Select(s.selectColumns...).
		From(sightingsTable).
		Where(sq.Eq{"id": id, "user_id": userID, "deleted_at": nil}).
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("build sightings get: %w", err)
	}

	var r sightingRow
	err = s.db.QueryRowxContext(ctx, query, args...).StructScan(&r)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	m := r.toModel()
	return &m, true, nil
}

// UpdateContent replaces the mutable content fields.
func (s *SightingStore) UpdateContent(ctx context.Context, id, userID string, upd models.SightingUpdate) (*models.Sighting, bool, error) {
	query, args, err := builder.
		Update(sightingsTable).
		SetMap(map[string]interface{}{
			"bird_id":           upd.BirdID,
			"quick_note":        upd.QuickNote,
			"notes":             upd.Notes,
			"photo_paths":       sq.Expr("?::text[]", StringArray(upd.PhotoPaths)),
			"recording_paths":   sq.Expr("?::text[]", StringArray(upd.RecordingPaths)),
			"client_updated_at": upd.ClientUpdatedAt,
			"updated_at":        sq.Expr("now()"),
		}).
		Where(sq.Eq{
			"id":         id,
			"user_id":    userID,
			"deleted_at": nil,
		}).
		Where(sq.LtOrEq{
			"client_updated_at": upd.ClientUpdatedAt,
		}).
		Suffix("RETURNING " + s.returningCols).
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("build sightings update: %w", err)
	}

	var r sightingRow
	err = s.db.QueryRowxContext(ctx, query, args...).StructScan(&r)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	m := r.toModel()
	return &m, true, nil
}
