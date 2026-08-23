//go:build bdd

// Package bdd is the birb acceptance-test suite: godog step definitions
// against a real chi router and Postgres. Run via `go test -tags bdd
// ./tests/bdd/...`; see tests/bdd/README.md for setup.
package bdd

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/jmoiron/sqlx"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/pressly/goose/v3"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/handler"
	"github.com/ivanw98/birb/internal/httpapi"
	"github.com/ivanw98/birb/internal/service"
	"github.com/ivanw98/birb/internal/store"
)

// Fixed issuer/audience/kid for the local JWKS server; not secrets since they only verify tokens against a throwaway RSA key generated per run.
const (
	testIssuer = "https://bdd.birb.test/auth/v1"
	testAud    = "authenticated"
	testKID    = "birb-bdd-kid"
)

// harness is the suite-scoped infrastructure shared by every scenario; scenarios execute sequentially and reset state between runs, so reuse is safe.
type harness struct {
	db      *sqlx.DB
	apiSrv  *httptest.Server
	jwksSrv *httptest.Server
	client  *http.Client
	baseURL string
	signer  *tokenSigner
}

func newHarness(ctx context.Context, dsn string) (*harness, error) {
	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", dsn, err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("goose dialect: %w", err)
	}
	migrationsDir := filepath.Join("..", "..", "db", "migrations")
	if err := goose.Up(db.DB, migrationsDir); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply migrations from %s: %w", migrationsDir, err)
	}

	signer, keys, err := newTokenSigner(testKID, testIssuer, testAud)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("build token signer: %w", err)
	}

	// A real httptest server serves the JWKS so NewJWKSVerifier exercises its real HTTP fetch path.
	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(keys)
	}))

	verifier, err := auth.NewJWKSVerifier(ctx, jwksSrv.URL, testIssuer, testAud)
	if err != nil {
		jwksSrv.Close()
		_ = db.Close()
		return nil, fmt.Errorf("build JWKS verifier: %w", err)
	}

	// Mirrors the composition root in cmd/api/main.go, minus config.Load (this harness doesn't read process env).
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	repos := store.New(db)
	sightingSvc := service.NewSightings(repos.Sightings, repos.Birds, log)
	birdSvc := service.NewBirds(repos.Birds)
	accountSvc := service.NewAccount(repos.Users)
	// A low failure limit so a scenario can trip the rate limiter in three requests
	// rather than twenty-one.
	groupSvc := service.NewGroups(repos.Groups, service.NewJoinLimiter(bddJoinFailureLimit, time.Hour))
	feedSvc := service.NewFeed(repos.Feed, repos.Groups)
	hdl := handler.New(sightingSvc, birdSvc, accountSvc, groupSvc, feedSvc, log)
	authn := auth.NewAuthenticator(verifier, repos.Users, log)

	apiSrv := httptest.NewServer(httpapi.NewRouter(hdl, authn, ""))

	return &harness{
		db:      db,
		apiSrv:  apiSrv,
		jwksSrv: jwksSrv,
		client:  apiSrv.Client(),
		baseURL: apiSrv.URL,
		signer:  signer,
	}, nil
}

func (h *harness) close() {
	h.apiSrv.Close()
	h.jwksSrv.Close()
	_ = h.db.Close()
}

// resetDB truncates per-scenario data between every scenario, keeping the
// migration-seeded birds reference list intact. Places are seeded by a migration too but
// are truncated anyway, so a scenario that cares about place names controls every
// candidate rather than competing with 5,000 real settlements.
func (h *harness) resetDB(ctx context.Context) error {
	_, err := h.db.ExecContext(ctx, `TRUNCATE public.group_members, public.groups, public.places, public.sightings, public.users RESTART IDENTITY CASCADE`)
	return err
}

// tokenSigner mints RS256 JWTs against a locally generated RSA key for the JWKS verifier to check end-to-end.
type tokenSigner struct {
	priv jwk.Key
	iss  string
	aud  string
}

// newTokenSigner generates a fresh RSA key and returns a signer over the
// private half plus a JWKS (public half only, tagged with kid+alg) ready to
// be served over HTTP.
func newTokenSigner(kid, iss, aud string) (*tokenSigner, jwk.Set, error) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate RSA key: %w", err)
	}

	priv, err := jwk.FromRaw(rsaKey)
	if err != nil {
		return nil, nil, fmt.Errorf("build private jwk: %w", err)
	}
	if err := priv.Set(jwk.KeyIDKey, kid); err != nil {
		return nil, nil, fmt.Errorf("set private kid: %w", err)
	}

	pub, err := jwk.FromRaw(rsaKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("build public jwk: %w", err)
	}
	if err := pub.Set(jwk.KeyIDKey, kid); err != nil {
		return nil, nil, fmt.Errorf("set public kid: %w", err)
	}
	if err := pub.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		return nil, nil, fmt.Errorf("set public alg: %w", err)
	}

	set := jwk.NewSet()
	if err := set.AddKey(pub); err != nil {
		return nil, nil, fmt.Errorf("add key to set: %w", err)
	}

	return &tokenSigner{priv: priv, iss: iss, aud: aud}, set, nil
}

// sign mints a JWT for the given subject.
func (s *tokenSigner) sign(sub, email string, displayName *string) (string, error) {
	b := jwt.NewBuilder().
		Issuer(s.iss).
		Audience([]string{s.aud}).
		Subject(sub).
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(time.Hour)).
		Claim("email", email)
	if displayName != nil {
		b = b.Claim("name", *displayName)
	}
	tok, err := b.Build()
	if err != nil {
		return "", fmt.Errorf("build token: %w", err)
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, s.priv))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return string(signed), nil
}
