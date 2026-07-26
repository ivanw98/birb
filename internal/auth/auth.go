// Package auth verifies Supabase-issued JWTs and provisions the caller's user record just-in-time.
package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/oklog/ulid/v2"

	"github.com/ivanw98/birb/internal/models"
)

// Claims is the subset of a verified token the app cares about.
type Claims struct {
	Subject     string // Supabase auth.users UUID (the `sub` claim)
	Email       string
	DisplayName *string
}

// TokenVerifier verifies a raw bearer token and returns its claims.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (*Claims, error)
}

// JWKSVerifier verifies tokens against a JWKS, checking issuer and audience.
type JWKSVerifier struct {
	issuer   string
	audience string
	keys     jwk.Set
}

var _ TokenVerifier = (*JWKSVerifier)(nil)

// NewJWKSVerifier builds a verifier that fetches and auto-refreshes the JWKS at
// jwksURL (e.g. https://<ref>.supabase.co/auth/v1/.well-known/jwks.json).
func NewJWKSVerifier(ctx context.Context, jwksURL, issuer, audience string) (*JWKSVerifier, error) {
	cache := jwk.NewCache(ctx)
	if err := cache.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, err
	}
	if _, err := cache.Refresh(ctx, jwksURL); err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}
	return &JWKSVerifier{issuer: issuer, audience: audience, keys: jwk.NewCachedSet(cache, jwksURL)}, nil
}

// newVerifierWithKeys is the test seam: a verifier over a static key set.
func newVerifierWithKeys(keys jwk.Set, issuer, audience string) *JWKSVerifier {
	return &JWKSVerifier{issuer: issuer, audience: audience, keys: keys}
}

// Verify parses and validates the token, returning its claims or an error.
func (v *JWKSVerifier) Verify(ctx context.Context, raw string) (*Claims, error) {
	opts := []jwt.ParseOption{
		jwt.WithKeySet(v.keys),
		jwt.WithValidate(true),
		jwt.WithContext(ctx),
	}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}
	if v.audience != "" {
		opts = append(opts, jwt.WithAudience(v.audience))
	}

	tok, err := jwt.Parse([]byte(raw), opts...)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if tok.Subject() == "" {
		return nil, fmt.Errorf("token missing sub claim")
	}

	c := &Claims{Subject: tok.Subject()}
	if v, ok := tok.Get("email"); ok {
		if s, ok := v.(string); ok {
			c.Email = s
		}
	}
	c.DisplayName = displayName(tok)
	return c, nil
}

// displayName extracts a human name from the common Supabase/OAuth claim
// locations, preferring a top-level name, then user_metadata.
func displayName(tok jwt.Token) *string {
	if v, ok := tok.Get("name"); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	if v, ok := tok.Get("user_metadata"); ok {
		if meta, ok := v.(map[string]any); ok {
			for _, key := range []string{"full_name", "name"} {
				if s, ok := meta[key].(string); ok && s != "" {
					return &s
				}
			}
		}
	}
	return nil
}

// generateUserID mints a fresh `usr_` prefixed lowercase ULID.
func generateUserID() string {
	return "usr_" + strings.ToLower(ulid.Make().String())
}

// ToUser projects verified claims onto a new user record (id filled by newID).
func (c *Claims) ToUser(id string) models.User {
	return models.User{ID: id, AuthID: c.Subject, Email: c.Email, DisplayName: c.DisplayName}
}
