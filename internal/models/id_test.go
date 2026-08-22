package models_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/models"
)

func TestNewIDRendersWirePattern(t *testing.T) {
	pattern := regexp.MustCompile(`^grp_[a-z0-9]{26}$`)
	id := models.NewID(models.PrefixGroup)
	require.Regexp(t, pattern, id.String())
	require.False(t, id.IsZero())
}

func TestNewIDIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		seen[models.NewID(models.PrefixUser).String()] = struct{}{}
	}
	require.Len(t, seen, 1000)
}

func TestTimeIsMintInstant(t *testing.T) {
	before := time.Now().Add(-time.Second)
	id := models.NewID(models.PrefixPlace)
	require.WithinRange(t, id.Time(), before, time.Now().Add(time.Second))
}

// An un-generated identifier must not render as a plausible all-zero ULID.
func TestZeroValueRendersEmpty(t *testing.T) {
	var id models.BirbID
	require.True(t, id.IsZero())
	require.Empty(t, id.String())
	require.True(t, id.Time().IsZero())
}

func TestValidIDAcceptsAMintedID(t *testing.T) {
	require.True(t, models.ValidID(models.PrefixSighting, models.NewID(models.PrefixSighting).String()))
}

func TestValidIDRejectsWrongEntity(t *testing.T) {
	require.False(t, models.ValidID(models.PrefixGroup, models.NewID(models.PrefixUser).String()))
}

func TestValidIDMatchesTheColumnConstraint(t *testing.T) {
	for name, tc := range map[string]struct {
		id   string
		want bool
	}{
		"too short":   {"grp_01j9z3x8k2m4n6p8r0s2t4v6w", false},
		"too long":    {"grp_01j9z3x8k2m4n6p8r0s2t4v6w88", false},
		"uppercase":   {"grp_01J9Z3X8K2M4N6P8R0S2T4V6W8", false},
		"no prefix":   {"01j9z3x8k2m4n6p8r0s2t4v6w8", false},
		"prefix only": {"grp_", false},
		"punctuation": {"grp_01j9z3x8k2m4n6p8r0s2t4v6w-", false},
		// The DB CHECK allows any [a-z0-9]; ulid.ParseStrict would reject this
		// leading 'f' as a timestamp overflow, so validation must not use it.
		"high leading character": {"grp_f1j9z3x8k2m4n6p8r0s2t4v6w8", true},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, models.ValidID(models.PrefixGroup, tc.id))
		})
	}
}
