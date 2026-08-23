package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/models"
)

var feedUser = models.User{ID: "usr_01j9z3x8k2m4n6p8r0s2t4v6w8"}

// newTestFeed pins the clock so the window is exactly assertable.
func newTestFeed(t *testing.T, now time.Time, authors []string) (*Feed, *mockFeedRepo) {
	t.Helper()
	feedRepo := &mockFeedRepo{}
	groupRepo := &mockGroupRepo{
		CoMemberIDsFn: func(context.Context, string) ([]string, error) { return authors, nil },
	}
	f := NewFeed(feedRepo, groupRepo)
	f.now = func() time.Time { return now }
	return f, feedRepo
}

func feedItemsAt(times ...time.Time) []models.FeedItem {
	out := make([]models.FeedItem, len(times))
	for i, ts := range times {
		out[i] = models.FeedItem{SightingID: models.NewID(models.PrefixSighting).String(), ObservedAt: ts}
	}
	return out
}

func TestWindowIsSevenDaysBackFromTheInjectedClock(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	f, repo := newTestFeed(t, now, []string{"usr_friend"})

	_, err := f.GetFeed(context.Background(), feedUser, 25, "")
	require.NoError(t, err)

	require.Equal(t, now, repo.GotUntil)
	require.Equal(t, now.Add(-7*24*time.Hour), repo.GotSince)
}

func TestWindowExcludesTheFuture(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	f, repo := newTestFeed(t, now, []string{"usr_friend"})

	_, err := f.GetFeed(context.Background(), feedUser, 25, "")
	require.NoError(t, err)

	require.False(t, repo.GotUntil.After(now), "until must never be later than now")
}

// The window moves with the clock, so a sighting drops out as it ages past seven days.
func TestWindowMovesWithTheClock(t *testing.T) {
	first := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	f, repo := newTestFeed(t, first, []string{"usr_friend"})

	_, err := f.GetFeed(context.Background(), feedUser, 25, "")
	require.NoError(t, err)
	firstSince := repo.GotSince

	f.now = func() time.Time { return first.Add(48 * time.Hour) }
	_, err = f.GetFeed(context.Background(), feedUser, 25, "")
	require.NoError(t, err)

	require.Equal(t, 48*time.Hour, repo.GotSince.Sub(firstSince))
}

func TestNoCoMembersReturnsAnEmptyPageWithoutQuerying(t *testing.T) {
	f, repo := newTestFeed(t, time.Now(), nil)

	page, err := f.GetFeed(context.Background(), feedUser, 25, "")
	require.NoError(t, err)
	require.NotNil(t, page.Items)
	require.Empty(t, page.Items)
	require.Nil(t, page.NextCursor)
	require.Zero(t, repo.Calls, "the expensive query must be skipped")
}

// A malformed cursor is a 400 whether or not the caller has friends.
// validation runs before any lookup so status cannot depend on unrelated state.
func TestMalformedCursorIsRejectedRegardlessOfMembership(t *testing.T) {
	for name, authors := range map[string][]string{
		"with co-members": {"usr_friend"},
		"with none":       nil,
	} {
		t.Run(name, func(t *testing.T) {
			f, repo := newTestFeed(t, time.Now(), authors)

			_, err := f.GetFeed(context.Background(), feedUser, 25, "not-a-cursor")
			require.Error(t, err)

			var coded *models.CodedError
			require.ErrorAs(t, err, &coded)
			require.Equal(t, models.CodeValidationFailed, coded.Code)
			require.Zero(t, repo.Calls)
		})
	}
}

func TestLimitIsClampedAndProbesOneExtraRow(t *testing.T) {
	for name, tc := range map[string]struct{ asked, wantFetched int }{
		"default when zero":   {0, 26},
		"honoured in range":   {10, 11},
		"clamped at maximum":  {500, 101},
		"default when absurd": {-3, 26},
	} {
		t.Run(name, func(t *testing.T) {
			f, repo := newTestFeed(t, time.Now(), []string{"usr_friend"})

			_, err := f.GetFeed(context.Background(), feedUser, tc.asked, "")
			require.NoError(t, err)
			require.Equal(t, tc.wantFetched, repo.GotLimit)
		})
	}
}

// The cursor must come from the last RETURNED row, not the probe row.
// Encoding the probe would start the next page one sighting too late.
func TestNextCursorComesFromTheLastReturnedRow(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rows := feedItemsAt(now.Add(-1*time.Hour), now.Add(-2*time.Hour), now.Add(-3*time.Hour))

	f, repo := newTestFeed(t, now, []string{"usr_friend"})
	repo.ListPageFn = func(context.Context, []string, time.Time, time.Time, *models.Cursor, int) ([]models.FeedItem, error) {
		return rows, nil
	}

	page, err := f.GetFeed(context.Background(), feedUser, 2, "")
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.NotNil(t, page.NextCursor)

	cur, err := models.DecodeCursor(*page.NextCursor)
	require.NoError(t, err)
	require.Equal(t, rows[1].SightingID, cur.ID, "cursor must be the second row, not the probe")
	require.Equal(t, rows[1].ObservedAt, cur.ObservedAt)
}

func TestNoNextCursorOnTheLastPage(t *testing.T) {
	now := time.Now()
	f, repo := newTestFeed(t, now, []string{"usr_friend"})
	repo.ListPageFn = func(context.Context, []string, time.Time, time.Time, *models.Cursor, int) ([]models.FeedItem, error) {
		return feedItemsAt(now.Add(-time.Hour)), nil
	}

	page, err := f.GetFeed(context.Background(), feedUser, 25, "")
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Nil(t, page.NextCursor)
}

// Items must serialise as [] rather than null, matching SightingPage.
func TestItemsAreNeverNil(t *testing.T) {
	f, repo := newTestFeed(t, time.Now(), []string{"usr_friend"})
	repo.ListPageFn = func(context.Context, []string, time.Time, time.Time, *models.Cursor, int) ([]models.FeedItem, error) {
		return nil, nil
	}

	page, err := f.GetFeed(context.Background(), feedUser, 25, "")
	require.NoError(t, err)
	require.NotNil(t, page.Items)
	require.Empty(t, page.Items)
}

func TestCoMemberIDsArePassedThroughToTheQuery(t *testing.T) {
	authors := []string{"usr_a", "usr_b"}
	f, repo := newTestFeed(t, time.Now(), authors)

	_, err := f.GetFeed(context.Background(), feedUser, 25, "")
	require.NoError(t, err)
	require.Equal(t, authors, repo.GotAuthors)
}
