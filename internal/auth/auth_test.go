package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer = "https://ref.supabase.co/auth/v1"
	testAud    = "authenticated"
	testKID    = "test-kid"
)

// signingHarness builds a JWKS + matching signer for verifier tests.
type signingHarness struct {
	priv jwk.Key
	set  jwk.Set
}

func newSigningHarness(t *testing.T) signingHarness {
	t.Helper()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	priv, err := jwk.FromRaw(rsaKey)
	require.NoError(t, err)
	require.NoError(t, priv.Set(jwk.KeyIDKey, testKID))

	pub, err := jwk.FromRaw(rsaKey.PublicKey)
	require.NoError(t, err)
	require.NoError(t, pub.Set(jwk.KeyIDKey, testKID))
	require.NoError(t, pub.Set(jwk.AlgorithmKey, jwa.RS256))

	set := jwk.NewSet()
	require.NoError(t, set.AddKey(pub))
	return signingHarness{priv: priv, set: set}
}

func (h signingHarness) sign(t *testing.T, build func(*jwt.Builder) *jwt.Builder) string {
	t.Helper()
	b := jwt.NewBuilder().
		Issuer(testIssuer).
		Audience([]string{testAud}).
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(time.Hour))
	tok, err := build(b).Build()
	require.NoError(t, err)
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, h.priv))
	require.NoError(t, err)
	return string(signed)
}

func TestJWKSVerifierValidToken(t *testing.T) {
	h := newSigningHarness(t)
	v := newVerifierWithKeys(h.set, testIssuer, testAud)
	raw := h.sign(t, func(b *jwt.Builder) *jwt.Builder {
		return b.Subject("auth-uuid-123").Claim("email", "birder@example.com")
	})

	claims, err := v.Verify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, "auth-uuid-123", claims.Subject)
	assert.Equal(t, "birder@example.com", claims.Email)
}

func TestJWKSVerifierDisplayNameFromMetadata(t *testing.T) {
	h := newSigningHarness(t)
	v := newVerifierWithKeys(h.set, testIssuer, testAud)

	topLevel := h.sign(t, func(b *jwt.Builder) *jwt.Builder {
		return b.Subject("u1").Claim("name", "Top Level")
	})
	c, err := v.Verify(context.Background(), topLevel)
	require.NoError(t, err)
	require.NotNil(t, c.DisplayName)
	assert.Equal(t, "Top Level", *c.DisplayName)

	meta := h.sign(t, func(b *jwt.Builder) *jwt.Builder {
		return b.Subject("u2").Claim("user_metadata", map[string]any{"full_name": "Jane Birder"})
	})
	c, err = v.Verify(context.Background(), meta)
	require.NoError(t, err)
	require.NotNil(t, c.DisplayName)
	assert.Equal(t, "Jane Birder", *c.DisplayName)
}

func TestJWKSVerifierRejects(t *testing.T) {
	h := newSigningHarness(t)

	t.Run("wrong issuer", func(t *testing.T) {
		v := newVerifierWithKeys(h.set, "https://evil.example", testAud)
		raw := h.sign(t, func(b *jwt.Builder) *jwt.Builder { return b.Subject("u") })
		_, err := v.Verify(context.Background(), raw)
		assert.Error(t, err)
	})

	t.Run("wrong audience", func(t *testing.T) {
		v := newVerifierWithKeys(h.set, testIssuer, "different-aud")
		raw := h.sign(t, func(b *jwt.Builder) *jwt.Builder { return b.Subject("u") })
		_, err := v.Verify(context.Background(), raw)
		assert.Error(t, err)
	})

	t.Run("expired", func(t *testing.T) {
		v := newVerifierWithKeys(h.set, testIssuer, testAud)
		tok, _ := jwt.NewBuilder().Issuer(testIssuer).Audience([]string{testAud}).
			Subject("u").Expiration(time.Now().Add(-time.Hour)).Build()
		signed, _ := jwt.Sign(tok, jwt.WithKey(jwa.RS256, h.priv))
		_, err := v.Verify(context.Background(), string(signed))
		assert.Error(t, err)
	})

	t.Run("missing sub", func(t *testing.T) {
		v := newVerifierWithKeys(h.set, testIssuer, testAud)
		raw := h.sign(t, func(b *jwt.Builder) *jwt.Builder { return b })
		_, err := v.Verify(context.Background(), raw)
		assert.Error(t, err)
	})

	t.Run("garbage token", func(t *testing.T) {
		v := newVerifierWithKeys(h.set, testIssuer, testAud)
		_, err := v.Verify(context.Background(), "not.a.jwt")
		assert.Error(t, err)
	})

	t.Run("signed by unknown key", func(t *testing.T) {
		other := newSigningHarness(t)
		v := newVerifierWithKeys(h.set, testIssuer, testAud) // trusts h, not other
		raw := other.sign(t, func(b *jwt.Builder) *jwt.Builder { return b.Subject("u") })
		_, err := v.Verify(context.Background(), raw)
		assert.Error(t, err)
	})
}

func TestGenerateUserIDShape(t *testing.T) {
	id := generateUserID()
	assert.Regexp(t, `^usr_[a-z0-9]{26}$`, id)
	assert.NotEqual(t, generateUserID(), id, "ids are unique")
}
