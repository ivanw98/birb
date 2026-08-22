package models

import (
	"regexp"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// Entity prefixes for BirbID. Stored values must match the CHECK regexes in db/migrations.
const (
	PrefixUser     = "usr_"
	PrefixSighting = "sgh_"
	PrefixBird     = "brd_"
	PrefixGroup    = "grp_"
	PrefixPlace    = "plc_"
)

// BirbID is a prefixed, k-sortable entity identifier: a type prefix and a lowercase ULID.
type BirbID struct {
	prefix string
	id     *ulid.ULID
}

// NewID mints a fresh identifier for the given entity prefix.
func NewID(prefix string) BirbID {
	id := ulid.Make()
	return BirbID{prefix: prefix, id: &id}
}

// idBody matches the 26-char lowercase ULID the CHECK constraints and the TypeSpec
// patterns both allow. Deliberately not ulid.ParseStrict, which also rejects a
// timestamp above 7 in the leading character and so is narrower than the column.
var idBody = regexp.MustCompile(`^[a-z0-9]{26}$`)

// ValidID reports whether s is a well-formed identifier for the given entity.
func ValidID(prefix, s string) bool {
	body, ok := strings.CutPrefix(s, prefix)
	return ok && idBody.MatchString(body)
}

// String renders the wire form: prefix + 26-char lowercase ULID. An un-generated identifier renders empty.
func (b BirbID) String() string {
	if b.id == nil {
		return ""
	}
	return b.prefix + strings.ToLower(b.id.String())
}

// Time is the instant the identifier was minted, to millisecond resolution; the zero time if un-generated.
func (b BirbID) Time() time.Time {
	if b.id == nil {
		return time.Time{}
	}
	return b.id.Timestamp()
}

// IsZero reports whether the identifier was never generated.
func (b BirbID) IsZero() bool {
	return b.id == nil
}
