package store

import (
	"context"
	"time"

	"github.com/ivanw98/birb/internal/models"
	"github.com/jmoiron/sqlx"
)

type FeedStore struct {
	db *sqlx.DB
}

func NewFeedStore(db *sqlx.DB) *FeedStore {
	return &FeedStore{db: db}
}

var _ FeedRepository = (*FeedStore)(nil)

const feedQuery = `
WITH page AS (
    SELECT s.id, s.user_id, s.bird_id, s.observed_at, s.latitude, s.longitude
    FROM sightings AS s
    WHERE s.user_id = ANY($1::text[])
      AND s.observed_at > $2::timestamptz
      AND s.observed_at <= $3::timestamptz
      AND s.deleted_at IS NULL
      AND ($4::timestamptz IS NULL OR (s.observed_at, s.id) < ($4::timestamptz, $5::text))
    ORDER BY s.observed_at DESC, s.id DESC
    LIMIT $6
)
SELECT p.id          AS sighting_id,
       p.bird_id,
       u.display_name AS author_name,
       p.observed_at,
       pl.name        AS place_name
FROM page AS p
JOIN users AS u ON u.id = p.user_id
LEFT JOIN LATERAL (
    SELECT c.name
    FROM places AS c
    WHERE p.latitude IS NOT NULL AND p.longitude IS NOT NULL
      AND c.population >= 500
      AND c.latitude BETWEEN p.latitude - 0.2695 AND p.latitude + 0.2695
      AND c.longitude BETWEEN p.longitude - (0.2695 / cos(radians(p.latitude)))
                          AND p.longitude + (0.2695 / cos(radians(p.latitude)))
      AND ((c.latitude - p.latitude)^2 + ((c.longitude - p.longitude) * cos(radians(p.latitude)))^2) <= 0.2695^2
    ORDER BY ((c.latitude - p.latitude)^2 + ((c.longitude - p.longitude) * cos(radians(p.latitude)))^2), c.geonames_id
    LIMIT 1
) AS pl ON true
ORDER BY p.observed_at DESC, p.id DESC
`

// feedRow is the wire projection.
type feedRow struct {
	SightingID string    `db:"sighting_id"`
	BirdID     *string   `db:"bird_id"`
	AuthorName *string   `db:"author_name"`
	ObservedAt time.Time `db:"observed_at"`
	PlaceName  *string   `db:"place_name"`
}

func (r feedRow) toModel() models.FeedItem {
	return models.FeedItem{
		SightingID: r.SightingID,
		BirdID:     r.BirdID,
		AuthorName: r.AuthorName,
		ObservedAt: r.ObservedAt.UTC(),
		PlaceName:  r.PlaceName,
	}
}

// ListPage returns a page of feed items for the given author IDs, between the given times.
func (s *FeedStore) ListPage(
	ctx context.Context,
	authorIDs []string,
	since, until time.Time,
	cursor *models.Cursor,
	limit int,
) ([]models.FeedItem, error) {
	var cursorAt *time.Time
	var cursorID *string
	if cursor != nil {
		cursorAt = &cursor.ObservedAt
		cursorID = &cursor.ID
	}

	args := []any{
		StringArray(authorIDs), since, until, cursorAt, cursorID, limit,
	}

	var rows []feedRow
	if err := s.db.SelectContext(ctx, &rows, feedQuery,
		args...); err != nil {
		return nil, err
	}

	out := make([]models.FeedItem, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}
