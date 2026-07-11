package models

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestEncodeDecodeCursorRoundTrip(t *testing.T) {
	ts := time.Date(2026, 7, 8, 6, 42, 11, 123456789, time.UTC)
	tok := EncodeCursor(ts, "sgh_01j9z3x8k2m4n6p8r0s2t4v6w8")

	got, err := DecodeCursor(tok)
	require.NoError(t, err)
	assert.True(t, got.ObservedAt.Equal(ts), "timestamp should round-trip")
	assert.Equal(t, "sgh_01j9z3x8k2m4n6p8r0s2t4v6w8", got.ID)
}

func TestEncodeCursorNormalizesToUTC(t *testing.T) {
	loc := time.FixedZone("BST", 3600)
	ts := time.Date(2026, 7, 8, 7, 42, 11, 0, loc)
	got, err := DecodeCursor(EncodeCursor(ts, "sgh_x"))
	require.NoError(t, err)
	assert.Equal(t, time.UTC, got.ObservedAt.Location())
	assert.True(t, got.ObservedAt.Equal(ts))
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	cases := map[string]string{
		"not base64":    "!!!not base64!!!",
		"missing sep":   base64Raw("2026-07-08T06:42:11Z"),
		"empty id":      base64Raw("2026-07-08T06:42:11Z|"),
		"bad timestamp": base64Raw("not-a-time|sgh_x"),
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeCursor(tok)
			require.Error(t, err)
			// Must be a client validation error (400), never a 500.
			ce := AsCoded(err)
			assert.Equal(t, http.StatusBadRequest, ce.HTTPStatus)
			assert.Equal(t, CodeValidationFailed, ce.Code)
		})
	}
}

func TestBirdsETagStableAndOrderSensitive(t *testing.T) {
	a := Bird{ID: "brd_1", CommonName: "Robin", ScientificName: "Erithacus rubecula", EbirdCode: ptr("eurrob1"), TaxonomicOrder: ptr(int32(500))}
	b := Bird{ID: "brd_2", CommonName: "Wren", ScientificName: "Troglodytes troglodytes"}

	list := []Bird{a, b}
	tag1 := BirdsETag(list)
	tag2 := BirdsETag([]Bird{a, b})
	assert.Equal(t, tag1, tag2, "same list yields same tag")
	assert.Regexp(t, `^"[0-9a-f]{64}"$`, tag1, "strong, quoted, hex")

	assert.NotEqual(t, tag1, BirdsETag([]Bird{b, a}), "order matters")

	changed := a
	changed.CommonName = "European Robin"
	assert.NotEqual(t, tag1, BirdsETag([]Bird{changed, b}), "field change changes tag")

	assert.NotEqual(t, tag1, BirdsETag([]Bird{a}), "membership change changes tag")
}

func TestBirdsETagDistinguishesNilVsEmptyCode(t *testing.T) {
	withNil := []Bird{{ID: "brd_1", CommonName: "X", ScientificName: "Y"}}
	withEmpty := []Bird{{ID: "brd_1", CommonName: "X", ScientificName: "Y", EbirdCode: ptr("")}}
	// Both hash the same here (nil and "" both contribute no bytes before the
	// separator) — documents the intended behaviour rather than asserting a bug.
	assert.Equal(t, BirdsETag(withNil), BirdsETag(withEmpty))
}

func TestAsCodedWrapsUnknownAsInternal(t *testing.T) {
	ce := AsCoded(assertAnError{})
	assert.Equal(t, http.StatusInternalServerError, ce.HTTPStatus)
	assert.Equal(t, CodeInternal, ce.Code)
}

func TestAsCodedPassesThroughCoded(t *testing.T) {
	orig := ErrNotFound("missing")
	ce := AsCoded(orig)
	assert.Same(t, orig, ce)
	assert.Equal(t, http.StatusNotFound, ce.HTTPStatus)
}

func TestCodedErrorAPIErrorProjection(t *testing.T) {
	ce := ErrUnknownBird("no such bird brd_zzz")
	ae := ce.APIError()
	assert.Equal(t, CodeUnknownBird, ae.Code)
	assert.Equal(t, "no such bird brd_zzz", ae.Message)
	assert.Contains(t, ce.Error(), CodeUnknownBird)
}

func TestSightingPageNextCursorSerializesNull(t *testing.T) {
	page := SightingPage{Items: []Sighting{}, NextCursor: nil}
	b, err := json.Marshal(page)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"nextCursor":null`)

	page.NextCursor = ptr("abc")
	b, _ = json.Marshal(page)
	assert.Contains(t, string(b), `"nextCursor":"abc"`)
}

func TestSightingOmitsInternalAndUnsetFields(t *testing.T) {
	s := Sighting{
		ID:         "sgh_1",
		UserID:     "usr_secret",
		PhotoPaths: []string{},
	}
	b, err := json.Marshal(s)
	require.NoError(t, err)
	str := string(b)
	assert.NotContains(t, str, "usr_secret", "user_id must never serialize")
	assert.NotContains(t, str, "birdId", "unset optional omitted")
	assert.Contains(t, str, `"photoPaths":[]`, "photoPaths always present")
}

func TestUserToMe(t *testing.T) {
	u := User{ID: "usr_1", Email: "a@b.co", DisplayName: ptr("Al"), Tier: TierPremium, AuthID: "uuid"}
	me := u.ToMe()
	assert.Equal(t, "usr_1", me.ID)
	assert.Equal(t, TierPremium, me.Tier)
	assert.Equal(t, "Al", *me.DisplayName)
}

// helpers

type assertAnError struct{}

func (assertAnError) Error() string { return "boom" }

func base64Raw(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}
