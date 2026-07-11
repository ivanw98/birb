package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/models"
)

// --- mock services ---

type mockSightingSvc struct {
	batch  func(ctx context.Context, u models.User, req models.BatchSyncRequest) (models.BatchSyncResponse, error)
	list   func(ctx context.Context, u models.User, limit int, cursor string) (models.SightingPage, error)
	update func(ctx context.Context, u models.User, id string, upd models.SightingUpdate) (models.Sighting, error)
}

func (m *mockSightingSvc) BatchSync(ctx context.Context, u models.User, req models.BatchSyncRequest) (models.BatchSyncResponse, error) {
	return m.batch(ctx, u, req)
}
func (m *mockSightingSvc) List(ctx context.Context, u models.User, limit int, cursor string) (models.SightingPage, error) {
	return m.list(ctx, u, limit, cursor)
}
func (m *mockSightingSvc) Update(ctx context.Context, u models.User, id string, upd models.SightingUpdate) (models.Sighting, error) {
	return m.update(ctx, u, id, upd)
}

type mockBirdSvc struct {
	list func(ctx context.Context) ([]models.Bird, string, error)
}

func (m *mockBirdSvc) List(ctx context.Context) ([]models.Bird, string, error) { return m.list(ctx) }

type mockAccountSvc struct {
	me func(ctx context.Context, authID string) (models.Me, error)
}

func (m *mockAccountSvc) Me(ctx context.Context, authID string) (models.Me, error) {
	return m.me(ctx, authID)
}

// --- harness ---

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

var user = models.User{ID: "usr_1", AuthID: "auth-uuid", Email: "a@b.co", Tier: models.TierFree}

// server mounts the handler behind middleware that injects `user` into the context, mimicking auth.Authenticator.
func server(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), user)))
		})
	})
	h.Register(r)
	return r
}

func do(t *testing.T, srv http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rdr)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func ptr[T any](v T) *T { return &v }

// --- BatchSync ---

func TestBatchSyncOK(t *testing.T) {
	m := &mockSightingSvc{batch: func(_ context.Context, u models.User, req models.BatchSyncRequest) (models.BatchSyncResponse, error) {
		assert.Equal(t, "usr_1", u.ID)
		require.Len(t, req.Sightings, 1)
		return models.BatchSyncResponse{Results: []models.BatchItemResult{{ID: req.Sightings[0].ID, Status: models.StatusCreated}}}, nil
	}}
	srv := server(New(m, nil, nil, testLogger()))

	rr := do(t, srv, http.MethodPost, "/sightings/batch", `{"sightings":[{"id":"sgh_1","observedAt":"2026-07-08T06:42:11Z","observedAtOffsetMinutes":60,"clientUpdatedAt":"2026-07-08T06:42:11Z"}]}`)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp models.BatchSyncResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, models.StatusCreated, resp.Results[0].Status)
}

func TestBatchSyncMalformedJSON(t *testing.T) {
	srv := server(New(&mockSightingSvc{}, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodPost, "/sightings/batch", `{not json`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), models.CodeBadRequest)
}

func TestBatchSyncOversizedBodyIs413(t *testing.T) {
	// The body must be rejected on size alone, before it reaches the service.
	m := &mockSightingSvc{batch: func(context.Context, models.User, models.BatchSyncRequest) (models.BatchSyncResponse, error) {
		t.Error("service must not be called for an oversized body")
		return models.BatchSyncResponse{}, nil
	}}
	srv := server(New(m, nil, nil, testLogger()))

	big := `{"sightings":[{"quickNote":"` + strings.Repeat("x", 1<<20) + `"}]}`
	rr := do(t, srv, http.MethodPost, "/sightings/batch", big)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Contains(t, rr.Body.String(), models.CodeBadRequest)
}

func TestBatchSyncTooLargeMapsTo400(t *testing.T) {
	m := &mockSightingSvc{batch: func(context.Context, models.User, models.BatchSyncRequest) (models.BatchSyncResponse, error) {
		return models.BatchSyncResponse{}, models.ErrBatchTooLarge("too many")
	}}
	srv := server(New(m, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodPost, "/sightings/batch", `{"sightings":[]}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), models.CodeBatchTooLarge)
}

// --- List ---

func TestListOK(t *testing.T) {
	next := "cursor123"
	m := &mockSightingSvc{list: func(_ context.Context, _ models.User, limit int, cursor string) (models.SightingPage, error) {
		assert.Equal(t, 10, limit)
		assert.Equal(t, "abc", cursor)
		return models.SightingPage{Items: []models.Sighting{{ID: "sgh_1", PhotoPaths: []string{}}}, NextCursor: &next}, nil
	}}
	srv := server(New(m, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodGet, "/sightings?limit=10&cursor=abc", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"nextCursor":"cursor123"`)
}

func TestListBadLimit(t *testing.T) {
	srv := server(New(&mockSightingSvc{}, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodGet, "/sightings?limit=abc", "")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), models.CodeValidationFailed)
}

func TestListPropagatesServiceError(t *testing.T) {
	m := &mockSightingSvc{list: func(context.Context, models.User, int, string) (models.SightingPage, error) {
		return models.SightingPage{}, models.ErrValidation("bad cursor")
	}}
	srv := server(New(m, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodGet, "/sightings?cursor=bad", "")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- Update ---

func TestUpdateOK(t *testing.T) {
	m := &mockSightingSvc{update: func(_ context.Context, _ models.User, id string, upd models.SightingUpdate) (models.Sighting, error) {
		assert.Equal(t, "sgh_1", id)
		assert.NotNil(t, upd.PhotoPaths)
		return models.Sighting{ID: id, PhotoPaths: []string{}}, nil
	}}
	srv := server(New(m, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodPut, "/sightings/sgh_1", `{"clientUpdatedAt":"2026-07-08T06:42:11Z","photoPaths":[]}`)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateStaleReturns409WithCurrent(t *testing.T) {
	current := &models.Sighting{ID: "sgh_1", ClientUpdatedAt: time.Now().UTC(), PhotoPaths: []string{}}
	m := &mockSightingSvc{update: func(context.Context, models.User, string, models.SightingUpdate) (models.Sighting, error) {
		return models.Sighting{}, &models.StaleError{Current: current}
	}}
	srv := server(New(m, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodPut, "/sightings/sgh_1", `{"clientUpdatedAt":"2020-01-01T00:00:00Z","photoPaths":[]}`)
	assert.Equal(t, http.StatusConflict, rr.Code)
	var body struct {
		Code    string          `json:"code"`
		Current models.Sighting `json:"current"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, models.CodeStaleUpdate, body.Code)
	assert.Equal(t, "sgh_1", body.Current.ID)
}

func TestUpdateNotFoundMapsTo404(t *testing.T) {
	m := &mockSightingSvc{update: func(context.Context, models.User, string, models.SightingUpdate) (models.Sighting, error) {
		return models.Sighting{}, models.ErrNotFound("nope")
	}}
	srv := server(New(m, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodPut, "/sightings/sgh_1", `{"clientUpdatedAt":"2026-07-08T06:42:11Z","photoPaths":[]}`)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdateMalformedJSON(t *testing.T) {
	srv := server(New(&mockSightingSvc{}, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodPut, "/sightings/sgh_1", `{bad`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdateInvalidPhotoPathMapsTo400(t *testing.T) {
	m := &mockSightingSvc{update: func(context.Context, models.User, string, models.SightingUpdate) (models.Sighting, error) {
		return models.Sighting{}, models.ErrInvalidPhotoPath("bad path")
	}}
	srv := server(New(m, nil, nil, testLogger()))
	rr := do(t, srv, http.MethodPut, "/sightings/sgh_1", `{"clientUpdatedAt":"2026-07-08T06:42:11Z","photoPaths":["x"]}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), models.CodeInvalidPhotoPath)
}

// --- Birds (ETag) ---

func birdSvc() *mockBirdSvc {
	birds := []models.Bird{{ID: "brd_1", CommonName: "Robin", ScientificName: "Erithacus rubecula"}}
	return &mockBirdSvc{list: func(context.Context) ([]models.Bird, string, error) {
		return birds, models.BirdsETag(birds), nil
	}}
}

func TestBirdsOKSetsETag(t *testing.T) {
	srv := server(New(nil, birdSvc(), nil, testLogger()))
	rr := do(t, srv, http.MethodGet, "/birds", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("ETag"))
	assert.Contains(t, rr.Body.String(), "Robin")
}

func TestBirdsConditional304(t *testing.T) {
	bs := birdSvc()
	_, etag, _ := bs.list(context.Background())
	srv := server(New(nil, bs, nil, testLogger()))

	req := httptest.NewRequest(http.MethodGet, "/birds", nil)
	req.Header.Set("If-None-Match", etag)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotModified, rr.Code)
	assert.Empty(t, rr.Body.String())
}

func TestBirdsStaleETagStill200(t *testing.T) {
	srv := server(New(nil, birdSvc(), nil, testLogger()))
	req := httptest.NewRequest(http.MethodGet, "/birds", nil)
	req.Header.Set("If-None-Match", `"deadbeef"`)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBirdsServiceError500(t *testing.T) {
	m := &mockBirdSvc{list: func(context.Context) ([]models.Bird, string, error) {
		return nil, "", models.ErrInternal("boom")
	}}
	srv := server(New(nil, m, nil, testLogger()))
	rr := do(t, srv, http.MethodGet, "/birds", "")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- Me ---

func TestMeOK(t *testing.T) {
	m := &mockAccountSvc{me: func(_ context.Context, authID string) (models.Me, error) {
		assert.Equal(t, "auth-uuid", authID)
		return models.Me{ID: "usr_1", Email: "a@b.co", Tier: models.TierFree, DisplayName: ptr("Al")}, nil
	}}
	srv := server(New(nil, nil, m, testLogger()))
	rr := do(t, srv, http.MethodGet, "/me", "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"tier":"free"`)
}

func TestMeServiceError(t *testing.T) {
	m := &mockAccountSvc{me: func(context.Context, string) (models.Me, error) {
		return models.Me{}, models.ErrInternal("db")
	}}
	srv := server(New(nil, nil, m, testLogger()))
	rr := do(t, srv, http.MethodGet, "/me", "")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestIfNoneMatchHelper(t *testing.T) {
	assert.False(t, ifNoneMatch("", `"a"`))
	assert.True(t, ifNoneMatch("*", `"a"`))
	assert.True(t, ifNoneMatch(`"a"`, `"a"`))
	assert.True(t, ifNoneMatch(`"x", "a"`, `"a"`))
	assert.True(t, ifNoneMatch(`W/"a"`, `"a"`))
	assert.False(t, ifNoneMatch(`"b"`, `"a"`))
}
