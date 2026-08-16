package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/models"
	"github.com/ivanw98/birb/internal/store"
)

var fixedNow = time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newSightingSvc(sr *mockSightingRepo, br *mockBirdRepo) *Sightings {
	s := NewSightings(sr, br, testLogger())
	s.now = func() time.Time { return fixedNow }
	return s
}

func ptr[T any](v T) *T { return &v }

const validSgh = "sgh_01j9z3x8k2m4n6p8r0s2t4v6w8"
const validBrd = "brd_01j9z3x8k2m4n6p8r0s2t4v6w8"
const testUserID = "usr_01j9z3x8k2m4n6p8r0s2t4v6w8"
const testAuthID = "11111111-1111-1111-1111-111111111111"

func testUser() models.User {
	return models.User{ID: testUserID, AuthID: testAuthID, Email: "a@b.co", Tier: models.TierFree}
}

func syncItem(id string) models.SightingSync {
	return models.SightingSync{
		ID:                      id,
		ObservedAt:              fixedNow.Add(-time.Hour),
		ObservedAtOffsetMinutes: 60,
		ClientUpdatedAt:         fixedNow.Add(-time.Hour),
	}
}

func TestBatchSyncTooLarge(t *testing.T) {
	svc := newSightingSvc(&mockSightingRepo{}, &mockBirdRepo{})
	req := models.BatchSyncRequest{Sightings: make([]models.SightingSync, 101)}
	_, err := svc.BatchSync(context.Background(), testUser(), req)
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeBatchTooLarge, ce.Code)
}

func TestBatchSyncMixedOutcomes(t *testing.T) {
	br := &mockBirdRepo{ExistingIDsFn: func(_ context.Context, ids []string) (map[string]struct{}, error) {
		return birdSet(validBrd), nil
	}}
	sr := &mockSightingRepo{UpsertFn: func(_ context.Context, s models.Sighting) (store.UpsertOutcome, error) {
		switch s.ID {
		case "sgh_00000000000000000000000001":
			return store.UpsertOutcome{Status: models.StatusCreated}, nil
		case "sgh_00000000000000000000000002":
			return store.UpsertOutcome{Status: models.StatusStale}, nil
		case "sgh_00000000000000000000000003":
			return store.UpsertOutcome{Conflict: true}, nil
		}
		return store.UpsertOutcome{}, errors.New("unexpected id")
	}}
	svc := newSightingSvc(sr, br)

	good := syncItem("sgh_00000000000000000000000001")
	good.BirdID = ptr(validBrd)
	stale := syncItem("sgh_00000000000000000000000002")
	conflict := syncItem("sgh_00000000000000000000000003")
	badID := syncItem("nope")
	future := syncItem("sgh_00000000000000000000000004")
	future.ObservedAt = fixedNow.Add(48 * time.Hour)
	unknownBird := syncItem("sgh_00000000000000000000000005")
	unknownBird.BirdID = ptr("brd_00000000000000000000000000") // not in set

	req := models.BatchSyncRequest{Sightings: []models.SightingSync{good, stale, conflict, badID, future, unknownBird}}
	resp, err := svc.BatchSync(context.Background(), testUser(), req)
	require.NoError(t, err)
	require.Len(t, resp.Results, 6)

	assert.Equal(t, models.StatusCreated, resp.Results[0].Status)
	assert.Equal(t, models.StatusStale, resp.Results[1].Status)
	assert.Equal(t, models.StatusInvalid, resp.Results[2].Status)
	assert.Equal(t, models.CodeIDConflict, resp.Results[2].Error.Code)
	assert.Equal(t, models.StatusInvalid, resp.Results[3].Status)
	assert.Equal(t, models.CodeValidationFailed, resp.Results[3].Error.Code)
	assert.Equal(t, models.CodeObservedInFuture, resp.Results[4].Error.Code)
	assert.Equal(t, models.CodeUnknownBird, resp.Results[5].Error.Code)
}

func TestBatchSyncBirdLookupError(t *testing.T) {
	br := &mockBirdRepo{ExistingIDsFn: func(_ context.Context, _ []string) (map[string]struct{}, error) {
		return nil, errors.New("db down")
	}}
	svc := newSightingSvc(&mockSightingRepo{}, br)
	_, err := svc.BatchSync(context.Background(), testUser(), models.BatchSyncRequest{Sightings: []models.SightingSync{syncItem(validSgh)}})
	require.Error(t, err)
}

func TestBatchSyncRepoErrorFailsRequest(t *testing.T) {
	br := &mockBirdRepo{ExistingIDsFn: func(_ context.Context, _ []string) (map[string]struct{}, error) { return birdSet(), nil }}
	sr := &mockSightingRepo{UpsertFn: func(_ context.Context, _ models.Sighting) (store.UpsertOutcome, error) {
		return store.UpsertOutcome{}, errors.New("conn reset")
	}}
	svc := newSightingSvc(sr, br)
	_, err := svc.BatchSync(context.Background(), testUser(), models.BatchSyncRequest{Sightings: []models.SightingSync{syncItem(validSgh)}})
	require.Error(t, err)
}

func TestListDefaultLimitAndNextCursor(t *testing.T) {
	// Return limit+1 rows to force a next cursor.
	var gotLimit int
	rows := make([]models.Sighting, defaultPageLimit+1)
	base := fixedNow
	for i := range rows {
		rows[i] = models.Sighting{ID: "sgh_" + pad(i), ObservedAt: base.Add(-time.Duration(i) * time.Minute)}
	}
	sr := &mockSightingRepo{ListFn: func(_ context.Context, _ string, _ *models.Cursor, limit int, _ bool) ([]models.Sighting, error) {
		gotLimit = limit
		return rows, nil
	}}
	svc := newSightingSvc(sr, &mockBirdRepo{})

	page, err := svc.List(context.Background(), testUser(), 0, "", false)
	require.NoError(t, err)
	assert.Equal(t, defaultPageLimit+1, gotLimit, "fetches limit+1")
	assert.Len(t, page.Items, defaultPageLimit)
	require.NotNil(t, page.NextCursor)
}

func TestListLastPageNilCursor(t *testing.T) {
	sr := &mockSightingRepo{ListFn: func(_ context.Context, _ string, _ *models.Cursor, _ int, _ bool) ([]models.Sighting, error) {
		return []models.Sighting{{ID: validSgh, ObservedAt: fixedNow}}, nil
	}}
	svc := newSightingSvc(sr, &mockBirdRepo{})
	page, err := svc.List(context.Background(), testUser(), 25, "", false)
	require.NoError(t, err)
	assert.Nil(t, page.NextCursor)
	assert.Len(t, page.Items, 1)
}

func TestListClampsLimit(t *testing.T) {
	var gotLimit int
	sr := &mockSightingRepo{ListFn: func(_ context.Context, _ string, _ *models.Cursor, limit int, _ bool) ([]models.Sighting, error) {
		gotLimit = limit
		return nil, nil
	}}
	svc := newSightingSvc(sr, &mockBirdRepo{})
	_, err := svc.List(context.Background(), testUser(), 9999, "", false)
	require.NoError(t, err)
	assert.Equal(t, maxPageLimit+1, gotLimit)
}

func TestListBadCursor(t *testing.T) {
	svc := newSightingSvc(&mockSightingRepo{}, &mockBirdRepo{})
	_, err := svc.List(context.Background(), testUser(), 25, "!!!not-valid!!!", false)
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeValidationFailed, ce.Code)
}

func TestListPassesDecodedCursor(t *testing.T) {
	token := models.EncodeCursor(fixedNow, validSgh)
	var got *models.Cursor
	sr := &mockSightingRepo{ListFn: func(_ context.Context, _ string, cur *models.Cursor, _ int, _ bool) ([]models.Sighting, error) {
		got = cur
		return nil, nil
	}}
	svc := newSightingSvc(sr, &mockBirdRepo{})
	_, err := svc.List(context.Background(), testUser(), 25, token, false)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, validSgh, got.ID)
}

func TestListPassesIncludeDeleted(t *testing.T) {
	var got bool
	sr := &mockSightingRepo{ListFn: func(_ context.Context, _ string, _ *models.Cursor, _ int, includeDeleted bool) ([]models.Sighting, error) {
		got = includeDeleted
		return nil, nil
	}}
	svc := newSightingSvc(sr, &mockBirdRepo{})
	_, err := svc.List(context.Background(), testUser(), 25, "", true)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestUpdateApplied(t *testing.T) {
	updated := &models.Sighting{ID: validSgh, UserID: testUserID}
	sr := &mockSightingRepo{UpdateFn: func(_ context.Context, _, _ string, _ models.SightingUpdate) (*models.Sighting, bool, error) {
		return updated, true, nil
	}}
	svc := newSightingSvc(sr, &mockBirdRepo{})
	got, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{ClientUpdatedAt: fixedNow, PhotoPaths: []string{}})
	require.NoError(t, err)
	assert.Equal(t, validSgh, got.ID)
}

func TestUpdateStaleReturnsStaleError(t *testing.T) {
	current := &models.Sighting{ID: validSgh, UserID: testUserID}
	sr := &mockSightingRepo{
		UpdateFn: func(_ context.Context, _, _ string, _ models.SightingUpdate) (*models.Sighting, bool, error) {
			return nil, false, nil
		},
		GetFn: func(_ context.Context, _, _ string) (*models.Sighting, bool, error) {
			return current, true, nil
		},
	}
	svc := newSightingSvc(sr, &mockBirdRepo{})
	_, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{ClientUpdatedAt: fixedNow, PhotoPaths: []string{}})
	var stale *models.StaleError
	require.ErrorAs(t, err, &stale)
	assert.Equal(t, current, stale.Current)
}

func TestUpdateNotFound(t *testing.T) {
	sr := &mockSightingRepo{
		UpdateFn: func(_ context.Context, _, _ string, _ models.SightingUpdate) (*models.Sighting, bool, error) {
			return nil, false, nil
		},
		GetFn: func(_ context.Context, _, _ string) (*models.Sighting, bool, error) { return nil, false, nil },
	}
	svc := newSightingSvc(sr, &mockBirdRepo{})
	_, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{ClientUpdatedAt: fixedNow, PhotoPaths: []string{}})
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeNotFound, ce.Code)
}

func TestUpdateRejectsForeignPhotoPath(t *testing.T) {
	svc := newSightingSvc(&mockSightingRepo{}, &mockBirdRepo{})
	// prefixed with a different auth uid
	bad := "22222222-2222-2222-2222-222222222222/" + validSgh + "/a.jpg"
	_, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{ClientUpdatedAt: fixedNow, PhotoPaths: []string{bad}})
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeInvalidPhotoPath, ce.Code)
}

func TestUpdateAcceptsOwnedPhotoPath(t *testing.T) {
	sr := &mockSightingRepo{UpdateFn: func(_ context.Context, _, _ string, _ models.SightingUpdate) (*models.Sighting, bool, error) {
		return &models.Sighting{ID: validSgh}, true, nil
	}}
	svc := newSightingSvc(sr, &mockBirdRepo{})
	ok := testAuthID + "/" + validSgh + "/photo_1.jpeg"
	_, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{ClientUpdatedAt: fixedNow, PhotoPaths: []string{ok}})
	require.NoError(t, err)
}

func TestUpdateUnknownBird(t *testing.T) {
	br := &mockBirdRepo{ExistingIDsFn: func(_ context.Context, _ []string) (map[string]struct{}, error) { return birdSet(), nil }}
	svc := newSightingSvc(&mockSightingRepo{}, br)
	_, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{
		ClientUpdatedAt: fixedNow, PhotoPaths: []string{}, BirdID: ptr(validBrd),
	})
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeUnknownBird, ce.Code)
}

func TestUpdateInvalidID(t *testing.T) {
	svc := newSightingSvc(&mockSightingRepo{}, &mockBirdRepo{})
	_, err := svc.Update(context.Background(), testUser(), "bogus", models.SightingUpdate{ClientUpdatedAt: fixedNow, PhotoPaths: []string{}})
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeValidationFailed, ce.Code)
}

func TestUpdateRejectsFutureClientUpdatedAt(t *testing.T) {
	// A clientUpdatedAt beyond the 24h skew grace would poison last-write-wins
	// for the row (every later edit compares stale), so the PUT must refuse it.
	svc := newSightingSvc(&mockSightingRepo{}, &mockBirdRepo{})
	_, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{
		ClientUpdatedAt: fixedNow.Add(25 * time.Hour),
		PhotoPaths:      []string{},
	})
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeClientTSInFuture, ce.Code)
	assert.Equal(t, http.StatusBadRequest, ce.HTTPStatus)

	// Just inside the grace window is accepted.
	sr := &mockSightingRepo{UpdateFn: func(_ context.Context, _, _ string, _ models.SightingUpdate) (*models.Sighting, bool, error) {
		return &models.Sighting{ID: validSgh}, true, nil
	}}
	svc = newSightingSvc(sr, &mockBirdRepo{})
	_, err = svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{
		ClientUpdatedAt: fixedNow.Add(23 * time.Hour),
		PhotoPaths:      []string{},
	})
	require.NoError(t, err)
}

func TestUpdateRejectsMissingClientUpdatedAt(t *testing.T) {
	svc := newSightingSvc(&mockSightingRepo{}, &mockBirdRepo{})
	_, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{PhotoPaths: []string{}})
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeValidationFailed, ce.Code)
	assert.Equal(t, http.StatusBadRequest, ce.HTTPStatus)
}

func TestUpdateRejectsOverlongContent(t *testing.T) {
	svc := newSightingSvc(&mockSightingRepo{}, &mockBirdRepo{})

	longNotes := strings.Repeat("x", 5001)
	_, err := svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{
		ClientUpdatedAt: fixedNow, Notes: &longNotes, PhotoPaths: []string{},
	})
	ce := models.AsCoded(err)
	assert.Equal(t, models.CodeValidationFailed, ce.Code)
	assert.Equal(t, http.StatusBadRequest, ce.HTTPStatus, "must be a 400, not a DB CHECK 500")

	longQuickNote := strings.Repeat("x", 281)
	_, err = svc.Update(context.Background(), testUser(), validSgh, models.SightingUpdate{
		ClientUpdatedAt: fixedNow, QuickNote: &longQuickNote, PhotoPaths: []string{},
	})
	assert.Equal(t, models.CodeValidationFailed, models.AsCoded(err).Code)
}

func TestBatchSyncRejectsMissingClientUpdatedAt(t *testing.T) {
	br := &mockBirdRepo{ExistingIDsFn: func(_ context.Context, _ []string) (map[string]struct{}, error) { return birdSet(), nil }}
	svc := newSightingSvc(&mockSightingRepo{}, br)

	item := syncItem("sgh_00000000000000000000000009")
	item.ClientUpdatedAt = time.Time{}
	resp, err := svc.BatchSync(context.Background(), testUser(), models.BatchSyncRequest{Sightings: []models.SightingSync{item}})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, models.StatusInvalid, resp.Results[0].Status)
	assert.Equal(t, models.CodeValidationFailed, resp.Results[0].Error.Code)
}

// pad renders i as a 26-char lowercase suffix so ids match ^sgh_[a-z0-9]{26}$.
func pad(i int) string {
	s := "0000000000000000000000000" + itoa(i)
	return s[len(s)-26:]
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
