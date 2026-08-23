package service

import (
	"context"
	"time"

	"github.com/ivanw98/birb/internal/models"
)

// GetFeed generates a feed page for the given user, with an optional cursor.
func (f *Feed) GetFeed(
	ctx context.Context,
	user models.User,
	limit int,
	cursor string,
) (models.FeedPage, error) {
	const feedWindow = 7 * 24 * time.Hour

	until := f.now()
	since := until.Add(-feedWindow)
	limit = clampLimit(limit)

	var cur *models.Cursor
	if cursor != "" {
		c, err := models.DecodeCursor(cursor)
		if err != nil {
			return models.FeedPage{}, err // validation CodedError → 400
		}

		cur = &c
	}

	authors, err := f.groups.CoMemberIDs(ctx, user.ID)
	if err != nil {
		return models.FeedPage{}, err
	}

	if len(authors) == 0 {
		return models.FeedPage{Items: []models.FeedItem{}, NextCursor: nil}, nil
	}

	// limit+1 is a probe, not a page: one extra row is how we know a next page exists
	// without a COUNT.
	rows, err := f.feed.ListPage(ctx, authors, since, until, cur, limit+1)
	if err != nil {
		return models.FeedPage{}, err
	}

	page := models.FeedPage{Items: rows, NextCursor: nil}
	if len(rows) > limit {
		last := rows[limit-1]
		page.Items = rows[:limit]
		next := models.EncodeCursor(last.ObservedAt, last.SightingID)
		page.NextCursor = &next
	}

	if page.Items == nil {
		page.Items = []models.FeedItem{}
	}

	return page, nil
}
