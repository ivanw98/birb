package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/models"
)

type fakeVerifier struct {
	claims *Claims
	err    error
}

func (f fakeVerifier) Verify(context.Context, string) (*Claims, error) { return f.claims, f.err }

type stubUserRepo struct {
	upsert func(ctx context.Context, u models.User) (models.User, error)
}

func (s stubUserRepo) Upsert(ctx context.Context, u models.User) (models.User, error) {
	return s.upsert(ctx, u)
}
func (s stubUserRepo) GetByAuthID(context.Context, string) (models.User, error) {
	return models.User{}, nil
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newAuth(v TokenVerifier, repo stubUserRepo) *Authenticator {
	a := NewAuthenticator(v, repo, testLogger())
	a.newID = func() string { return "usr_00000000000000000000000000" }
	return a
}

func doRequest(t *testing.T, a *Authenticator, authHeader string, next http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rr := httptest.NewRecorder()
	a.Middleware(next).ServeHTTP(rr, req)
	return rr
}

func TestMiddlewareInjectsProvisionedUser(t *testing.T) {
	repo := stubUserRepo{upsert: func(_ context.Context, u models.User) (models.User, error) {
		u.Tier = models.TierFree
		return u, nil
	}}
	a := newAuth(fakeVerifier{claims: &Claims{Subject: "auth-1", Email: "a@b.co"}}, repo)

	var seen models.User
	rr := doRequest(t, a, "Bearer good.token", func(w http.ResponseWriter, r *http.Request) {
		seen = MustUser(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "auth-1", seen.AuthID)
	assert.Equal(t, "usr_00000000000000000000000000", seen.ID)
	assert.Equal(t, "a@b.co", seen.Email)
}

func TestMiddlewareRejectsMissingHeader(t *testing.T) {
	a := newAuth(fakeVerifier{}, stubUserRepo{})
	called := false
	rr := doRequest(t, a, "", func(http.ResponseWriter, *http.Request) { called = true })
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, called)
	assert.Contains(t, rr.Body.String(), models.CodeUnauthorized)
}

func TestMiddlewareRejectsNonBearer(t *testing.T) {
	a := newAuth(fakeVerifier{}, stubUserRepo{})
	rr := doRequest(t, a, "Basic abc", func(http.ResponseWriter, *http.Request) {})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMiddlewareRejectsBadToken(t *testing.T) {
	a := newAuth(fakeVerifier{err: errors.New("expired")}, stubUserRepo{})
	rr := doRequest(t, a, "Bearer x", func(http.ResponseWriter, *http.Request) {})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMiddlewareProvisionFailureIs500(t *testing.T) {
	repo := stubUserRepo{upsert: func(context.Context, models.User) (models.User, error) {
		return models.User{}, errors.New("db down")
	}}
	a := newAuth(fakeVerifier{claims: &Claims{Subject: "auth-1"}}, repo)
	rr := doRequest(t, a, "Bearer good", func(http.ResponseWriter, *http.Request) {})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), models.CodeInternal)
}

func TestBearerTokenParsing(t *testing.T) {
	cases := map[string]struct {
		header string
		ok     bool
	}{
		"valid":            {"Bearer abc", true},
		"case insensitive": {"bearer abc", true},
		"empty":            {"", false},
		"no prefix":        {"abc", false},
		"prefix only":      {"Bearer ", false},
		"spaces only":      {"Bearer    ", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.header != "" {
				r.Header.Set("Authorization", c.header)
			}
			tok, err := bearerToken(r)
			if c.ok {
				require.NoError(t, err)
				assert.Equal(t, "abc", tok)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestUserContextHelpers(t *testing.T) {
	_, ok := UserFrom(context.Background())
	assert.False(t, ok)

	ctx := WithUser(context.Background(), models.User{ID: "usr_1"})
	got, ok := UserFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, "usr_1", got.ID)
	assert.Equal(t, "usr_1", MustUser(ctx).ID)

	assert.Panics(t, func() { MustUser(context.Background()) })
}
