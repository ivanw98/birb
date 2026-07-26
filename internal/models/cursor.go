package models

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Cursor is the opaque keyset pagination cursor for the sightings feed, ordered by (observed_at, id) descending.
type Cursor struct {
	ObservedAt time.Time
	ID         string
}

// EncodeCursor renders a cursor as a URL-safe opaque token.
func EncodeCursor(observedAt time.Time, id string) string {
	raw := observedAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a token produced by EncodeCursor, returning a validation error (never 500) for malformed input.
func DecodeCursor(token string) (Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, ErrValidation("cursor is not valid base64url")
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		return Cursor{}, ErrValidation("cursor is malformed")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return Cursor{}, ErrValidation("cursor timestamp is invalid")
	}
	return Cursor{ObservedAt: ts.UTC(), ID: parts[1]}, nil
}

// String implements fmt.Stringer for debugging/logging.
func (c Cursor) String() string {
	return fmt.Sprintf("Cursor(%s, %s)", c.ObservedAt.Format(time.RFC3339Nano), c.ID)
}
