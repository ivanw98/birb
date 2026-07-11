package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/handler"
	"github.com/ivanw98/birb/internal/httpapi"
	"github.com/ivanw98/birb/internal/models"
)

// failVerifier rejects everything, so /api requires auth and returns 401.
type failVerifier struct{}

func (failVerifier) Verify(context.Context, string) (*auth.Claims, error) {
	return nil, assert.AnError
}

type nopUserRepo struct{}

func (nopUserRepo) Upsert(context.Context, models.User) (models.User, error) {
	return models.User{}, nil
}
func (nopUserRepo) GetByAuthID(context.Context, string) (models.User, error) {
	return models.User{}, nil
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRouterHealthzIsPublic(t *testing.T) {
	h := handler.New(nil, nil, nil, testLogger())
	authn := auth.NewAuthenticator(failVerifier{}, nopUserRepo{}, testLogger())
	srv := httptest.NewServer(httpapi.NewRouter(h, authn))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(body))
}

func TestRouterAPIRequiresAuth(t *testing.T) {
	h := handler.New(nil, nil, nil, testLogger())
	authn := auth.NewAuthenticator(failVerifier{}, nopUserRepo{}, testLogger())
	srv := httptest.NewServer(httpapi.NewRouter(h, authn))
	defer srv.Close()

	// No Authorization header → the auth middleware rejects before the handler.
	resp, err := http.Get(srv.URL + "/api/me")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
