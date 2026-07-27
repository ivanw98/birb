package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/models"
)

func newMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	//nolint:errcheck // best effort
	t.Cleanup(func() { db.Close() })
	return sqlx.NewDb(db, "sqlmock"), mock
}

func ptr[T any](v T) *T { return &v }

func sightingCols() []string {
	return []string{
		"id", "user_id", "observed_at", "observed_at_offset_minutes", "client_updated_at",
		"created_at", "updated_at", "bird_id", "quick_note", "notes", "latitude", "longitude",
		"accuracy_m", "photo_paths",
	}
}

func sightingRowValues(id, userID string, ts time.Time) []driver.Value {
	return []driver.Value{
		id, userID, ts, int32(60), ts, ts, ts, nil, ptr("note"), nil, nil, nil, nil, "{}",
	}
}

// --- UserStore ---

func TestUserStoreUpsert(t *testing.T) {
	db, mock := newMock(t)
	s := NewUserStore(db)
	now := time.Now().UTC()

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("usr_1", "auth-uuid", "a@b.co", (*string)(nil)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "auth_id", "email", "display_name", "tier", "created_at", "updated_at"}).
			AddRow("usr_1", "auth-uuid", "a@b.co", nil, "free", now, now))

	got, err := s.Upsert(context.Background(), models.User{ID: "usr_1", AuthID: "auth-uuid", Email: "a@b.co"})
	require.NoError(t, err)
	assert.Equal(t, "usr_1", got.ID)
	assert.Equal(t, models.TierFree, got.Tier)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserStoreGetByAuthIDNotFound(t *testing.T) {
	db, mock := newMock(t)
	s := NewUserStore(db)
	mock.ExpectQuery(`SELECT .* FROM users WHERE auth_id`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := s.GetByAuthID(context.Background(), "missing")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// --- BirdStore ---

func TestBirdStoreList(t *testing.T) {
	db, mock := newMock(t)
	s := NewBirdStore(db)
	mock.ExpectQuery(`SELECT .* FROM birds`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "common_name", "scientific_name", "ebird_code", "taxonomic_order"}).
			AddRow("brd_1", "Robin", "Erithacus rubecula", "eurrob1", 500).
			AddRow("brd_2", "Wren", "Troglodytes troglodytes", nil, nil))

	got, err := s.List(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Robin", got[0].CommonName)
	assert.Nil(t, got[1].EbirdCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBirdStoreExistingIDsEmptySkipsQuery(t *testing.T) {
	db, _ := newMock(t)
	s := NewBirdStore(db)
	got, err := s.ExistingIDs(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestBirdStoreExistingIDs(t *testing.T) {
	db, mock := newMock(t)
	s := NewBirdStore(db)

	expectedSQL := "SELECT id FROM birds WHERE id IN ($1,$2,$3)"

	mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
		WithArgs("brd_1", "brd_2", "brd_3").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("brd_1").AddRow("brd_3"))

	got, err := s.ExistingIDs(context.Background(), []string{"brd_1", "brd_2", "brd_3"})
	require.NoError(t, err)

	assert.Contains(t, got, "brd_1")
	assert.Contains(t, got, "brd_3")
	assert.NotContains(t, got, "brd_2")
}

// --- SightingStore.Upsert branches ---

func TestSightingUpsertCreated(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	mock.ExpectQuery(`INSERT INTO sightings`).
		WillReturnRows(sqlmock.NewRows([]string{"inserted"}).AddRow(true))

	out, err := s.Upsert(context.Background(), models.Sighting{ID: "sgh_1", UserID: "usr_1", ClientUpdatedAt: time.Now()})
	require.NoError(t, err)
	assert.Equal(t, models.StatusCreated, out.Status)
	assert.False(t, out.Conflict)
}

func TestSightingUpsertUpdated(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	mock.ExpectQuery(`INSERT INTO sightings`).
		WillReturnRows(sqlmock.NewRows([]string{"inserted"}).AddRow(false))

	out, err := s.Upsert(context.Background(), models.Sighting{ID: "sgh_1", UserID: "usr_1", ClientUpdatedAt: time.Now()})
	require.NoError(t, err)
	assert.Equal(t, models.StatusUpdated, out.Status)
}

func TestSightingUpsertStaleWhenOwnedBySelf(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	// Empty RETURNING → classify with follow-up SELECT that finds our own row.
	mock.ExpectQuery(`INSERT INTO sightings`).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT user_id FROM sightings WHERE id`).
		WithArgs("sgh_1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("usr_1"))

	out, err := s.Upsert(context.Background(), models.Sighting{ID: "sgh_1", UserID: "usr_1", ClientUpdatedAt: time.Now()})
	require.NoError(t, err)
	assert.Equal(t, models.StatusStale, out.Status)
	assert.False(t, out.Conflict)
}

func TestSightingUpsertConflictWhenOwnedByOther(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	mock.ExpectQuery(`INSERT INTO sightings`).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT user_id FROM sightings WHERE id`).
		WithArgs("sgh_1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("usr_OTHER"))

	out, err := s.Upsert(context.Background(), models.Sighting{ID: "sgh_1", UserID: "usr_1", ClientUpdatedAt: time.Now()})
	require.NoError(t, err)
	assert.True(t, out.Conflict)
}

// --- SightingStore.ListByUser ---

func TestSightingListByUserWithCursor(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	now := time.Now().UTC()
	cur := &models.Cursor{ObservedAt: now, ID: "sgh_z"}

	mock.ExpectQuery(regexp.QuoteMeta(`(observed_at, id) < (`)).
		WillReturnRows(sqlmock.NewRows(sightingCols()).
			AddRow(sightingRowValues("sgh_1", "usr_1", now)...))

	got, err := s.ListByUser(context.Background(), "usr_1", cur, 25)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "sgh_1", got[0].ID)
	assert.Equal(t, []string{}, got[0].PhotoPaths, "empty array normalizes to non-nil")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSightingListByUserNoCursor(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT .* FROM sightings WHERE deleted_at IS NULL AND user_id`).
		WillReturnRows(sqlmock.NewRows(sightingCols()))

	got, err := s.ListByUser(context.Background(), "usr_1", nil, 25)
	require.NoError(t, err)
	assert.Empty(t, got)
	_ = now
}

// --- SightingStore.GetForUser ---

func TestGetForUserFound(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT .* FROM sightings WHERE deleted_at IS NULL AND id = .* AND user_id`).
		WithArgs("sgh_1", "usr_1").
		WillReturnRows(sqlmock.NewRows(sightingCols()).AddRow(sightingRowValues("sgh_1", "usr_1", now)...))

	got, found, err := s.GetForUser(context.Background(), "sgh_1", "usr_1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "sgh_1", got.ID)
}

func TestGetForUserNotFound(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	mock.ExpectQuery(`SELECT .* FROM sightings WHERE deleted_at IS NULL AND id`).
		WithArgs("sgh_x", "usr_1").
		WillReturnError(sql.ErrNoRows)

	got, found, err := s.GetForUser(context.Background(), "sgh_x", "usr_1")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, got)
}

// --- SightingStore.UpdateContent ---

func TestUpdateContentApplied(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	now := time.Now().UTC()
	mock.ExpectQuery(`UPDATE sightings`).
		WillReturnRows(sqlmock.NewRows(sightingCols()).AddRow(sightingRowValues("sgh_1", "usr_1", now)...))

	got, applied, err := s.UpdateContent(context.Background(), "sgh_1", "usr_1", models.SightingUpdate{
		ClientUpdatedAt: now, PhotoPaths: []string{"uid/sgh_1/a.jpg"},
	})
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, "sgh_1", got.ID)
}

func TestUpdateContentNotAppliedWhenNoRow(t *testing.T) {
	db, mock := newMock(t)
	s := NewSightingStore(db)
	mock.ExpectQuery(`UPDATE sightings`).WillReturnError(sql.ErrNoRows)

	got, applied, err := s.UpdateContent(context.Background(), "sgh_1", "usr_1", models.SightingUpdate{
		ClientUpdatedAt: time.Now(), PhotoPaths: []string{},
	})
	require.NoError(t, err)
	assert.False(t, applied)
	assert.Nil(t, got)
}
