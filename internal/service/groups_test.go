package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormaliseJoinCode(t *testing.T) {
	for name, tc := range map[string]struct {
		in   string
		want string
		ok   bool
	}{
		"canonical":         {"BQTX7RKM", "BQTX7RKM", true},
		"lowercase":         {"bqtx7rkm", "BQTX7RKM", true},
		"hyphenated":        {"BQTX-7RKM", "BQTX7RKM", true},
		"underscored":       {"BQTX_7RKM", "BQTX7RKM", true},
		"spaced and padded": {"  bqtx 7rkm  ", "BQTX7RKM", true},
		"too short":         {"BQTX7RK", "", false},
		"too long":          {"BQTX7RKMM", "", false},
		"empty":             {"", "", false},
		"separators only":   {"----", "", false},
		"excluded letter O": {"BQTX7RKO", "", false},
		"excluded letter I": {"BQTX7RKI", "", false},
		"excluded letter L": {"BQTX7RKL", "", false},
		"excluded letter U": {"BQTX7RKU", "", false},
		"excluded digit 0":  {"BQTX7RK0", "", false},
		"excluded digit 1":  {"BQTX7RK1", "", false},
		"non-alphanumeric":  {"BQTX7RK!", "", false},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := normaliseJoinCode(tc.in)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

// An excluded character must be refused, not dropped: silently deleting the O from
// BQTXO7RKM would leave a different, well-formed code and look up the wrong group.
func TestNormaliseJoinCodeDoesNotSilentlyShiftAnExcludedCharacter(t *testing.T) {
	_, ok := normaliseJoinCode("BQTXO7RKM")
	require.False(t, ok)
}

func TestNewJoinCodeShape(t *testing.T) {
	for range 200 {
		code, err := newJoinCode()
		require.NoError(t, err)
		require.Len(t, code, joinCodeLen)
		for _, r := range code {
			require.True(t, strings.ContainsRune(joinCodeAlphabet, r), "unexpected character %q", r)
		}
	}
}

func TestJoinLimiterBlocksAtTheLimit(t *testing.T) {
	l := NewJoinLimiter(3, time.Hour)

	for range 3 {
		require.False(t, l.blocked("usr_a"))
		l.fail("usr_a")
	}
	require.True(t, l.blocked("usr_a"))
}

func TestJoinLimiterIsPerUser(t *testing.T) {
	l := NewJoinLimiter(1, time.Hour)
	l.fail("usr_a")

	require.True(t, l.blocked("usr_a"))
	require.False(t, l.blocked("usr_b"))
}

// The window is the only thing that releases a blocked caller, and nothing in a feature
// file can advance the clock.
func TestJoinLimiterReleasesAfterTheWindow(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	l := NewJoinLimiter(2, time.Hour)
	l.now = func() time.Time { return now }

	l.fail("usr_a")
	l.fail("usr_a")
	require.True(t, l.blocked("usr_a"))

	now = now.Add(59 * time.Minute)
	require.True(t, l.blocked("usr_a"), "still inside the window")

	now = now.Add(2 * time.Minute)
	require.False(t, l.blocked("usr_a"), "window has passed")
}

// Attempts age out one at a time, so a caller regains exactly the slots that expired.
func TestJoinLimiterExpiresAttemptsIndividually(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	l := NewJoinLimiter(2, time.Hour)
	l.now = func() time.Time { return now }

	l.fail("usr_a")
	now = now.Add(30 * time.Minute)
	l.fail("usr_a")
	require.True(t, l.blocked("usr_a"))

	// The first attempt has aged out, the second has not.
	now = now.Add(31 * time.Minute)
	require.False(t, l.blocked("usr_a"))
	l.fail("usr_a")
	require.True(t, l.blocked("usr_a"))
}

// A caller already at the limit must not keep growing their slice for the rest of the window.
func TestJoinLimiterCapsStoredAttempts(t *testing.T) {
	l := NewJoinLimiter(2, time.Hour)
	for range 100 {
		l.fail("usr_a")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	require.LessOrEqual(t, len(l.attempts["usr_a"]), 2)
}

// Idle users must not accumulate in the map forever.
func TestJoinLimiterSweepsExpiredUsers(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	l := NewJoinLimiter(1, time.Minute)
	l.now = func() time.Time { return now }

	// One short of the sweep interval, so the next failure is the one that triggers it.
	for i := range sweepEvery - 1 {
		l.fail(fmt.Sprintf("usr_%d", i))
	}
	l.mu.Lock()
	before := len(l.attempts)
	l.mu.Unlock()
	require.Equal(t, sweepEvery-1, before)

	now = now.Add(2 * time.Minute)
	l.fail("usr_trigger")

	l.mu.Lock()
	after := len(l.attempts)
	l.mu.Unlock()
	require.Equal(t, 1, after, "every aged-out user should have been swept")
}
