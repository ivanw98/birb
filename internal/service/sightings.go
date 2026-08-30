package service

import (
	"context"

	"github.com/ivanw98/birb/internal/models"
)

const (
	defaultPageLimit = 25
	maxPageLimit     = 100
)

// BatchSync validates each item and applies it via the repository; a bad item becomes an `invalid` result rather than failing the whole batch.
func (s *Sightings) BatchSync(ctx context.Context, user models.User, req models.BatchSyncRequest) (models.BatchSyncResponse, error) {
	if len(req.Sightings) > maxBatchItems {
		return models.BatchSyncResponse{}, models.ErrBatchTooLarge("batch exceeds 100 items; chunk the request")
	}

	// Bulk-resolve referenced bird ids so validation is one query, not N.
	birdExists, err := s.birds.ExistingIDs(ctx, distinctBirdIDs(req.Sightings))
	if err != nil {
		return models.BatchSyncResponse{}, err
	}

	results := make([]models.BatchItemResult, 0, len(req.Sightings))
	for _, item := range req.Sightings {
		if apiError := s.validateSyncItem(item, birdExists); apiError != nil {
			results = append(results, models.BatchItemResult{ID: item.ID, Status: models.StatusInvalid, Error: apiError})
			continue
		}

		outcome, err := s.sightings.Upsert(ctx, toSighting(user.ID, item))
		if err != nil {
			// Infra errors fail the whole request; safe since Upsert is idempotent and the client retries.
			return models.BatchSyncResponse{}, err
		}
		if outcome.Conflict {
			results = append(results, models.BatchItemResult{
				ID:     item.ID,
				Status: models.StatusInvalid,
				Error:  apiErr(models.CodeIDConflict, "id already exists for another user"),
			})
			continue
		}
		results = append(results, models.BatchItemResult{ID: item.ID, Status: outcome.Status})
	}
	return models.BatchSyncResponse{Results: results}, nil
}

// List returns a page of the user's sightings using keyset pagination, fetching one extra row to detect a next page.
func (s *Sightings) List(ctx context.Context, user models.User, limit int, cursor string, includeDeleted bool) (models.SightingPage, error) {
	limit = clampLimit(limit)

	var cur *models.Cursor
	if cursor != "" {
		c, err := models.DecodeCursor(cursor)
		if err != nil {
			return models.SightingPage{}, err // validation CodedError → 400
		}
		cur = &c
	}

	rows, err := s.sightings.ListByUser(ctx, user.ID, cur, limit+1, includeDeleted)
	if err != nil {
		return models.SightingPage{}, err
	}

	page := models.SightingPage{Items: rows, NextCursor: nil}
	if len(rows) > limit {
		last := rows[limit-1]
		page.Items = rows[:limit]
		next := models.EncodeCursor(last.ObservedAt, last.ID)
		page.NextCursor = &next
	}
	if page.Items == nil {
		page.Items = []models.Sighting{}
	}
	return page, nil
}

// Update enriches a sighting (full replace of content fields), enforcing media
// ownership and last-write-wins with a loud 409 on a stale write.
func (s *Sightings) Update(
	ctx context.Context,
	user models.User,
	id string,
	upd models.SightingUpdate,
) (models.Sighting, error) {
	if err := s.validateUpdate(ctx, user, id, upd); err != nil {
		return models.Sighting{}, err
	}

	row, applied, err := s.sightings.UpdateContent(ctx, id, user.ID, upd)
	if err != nil {
		return models.Sighting{}, err
	}
	if applied {
		return *row, nil
	}

	return s.resolveUnappliedUpdate(ctx, id, user.ID)
}

func (s *Sightings) validateUpdate(
	ctx context.Context,
	user models.User,
	id string,
	upd models.SightingUpdate,
) error {
	if !sightingIDPattern.MatchString(id) {
		return models.ErrValidation("id must match ^sgh_[a-z0-9]{26}$")
	}

	if ae := s.validateClientUpdatedAt(upd.ClientUpdatedAt); ae != nil {
		return asBadRequest(ae)
	}
	if ae := validateContentLengths(upd.QuickNote, upd.Notes); ae != nil {
		return asBadRequest(ae)
	}

	if err := validateSightingMedia(user.AuthID, id, upd); err != nil {
		return err
	}

	return s.validateBird(ctx, upd.BirdID)
}

func validateSightingMedia(
	authID string,
	sightingID string,
	upd models.SightingUpdate,
) error {
	if len(upd.PhotoPaths) > maxPhotoPaths {
		return models.ErrValidation("at most 10 photos")
	}
	if len(upd.RecordingPaths) > maxRecordingPaths {
		return models.ErrValidation("at most 5 recordings")
	}

	photoPathPattern := mediaPathRegex(authID, sightingID, photoFilePattern)
	for _, path := range upd.PhotoPaths {
		if len(path) > maxMediaPathLen {
			return models.ErrInvalidPhotoPath("photo path exceeds 512 characters")
		}
		if !photoPathPattern.MatchString(path) {
			return models.ErrInvalidPhotoPath("photo path not owned by caller: " + path)
		}
	}

	recordingPathPattern := mediaPathRegex(authID, sightingID, recordingFilePattern)
	for _, path := range upd.RecordingPaths {
		if len(path) > maxMediaPathLen {
			return models.ErrInvalidRecordingPath("recording path exceeds 512 characters")
		}
		if !recordingPathPattern.MatchString(path) {
			return models.ErrInvalidRecordingPath("recording path not owned by caller: " + path)
		}
	}

	return nil
}

func (s *Sightings) validateBird(ctx context.Context, birdID *string) error {
	if birdID == nil {
		return nil
	}

	if !birdIDPattern.MatchString(*birdID) {
		return models.ErrValidation("birdId must match ^brd_[a-z0-9]{26}$")
	}

	existing, err := s.birds.ExistingIDs(ctx, []string{*birdID})
	if err != nil {
		return err
	}
	if _, ok := existing[*birdID]; !ok {
		return models.ErrUnknownBird("birdId does not exist: " + *birdID)
	}

	return nil
}

func (s *Sightings) resolveUnappliedUpdate(
	ctx context.Context,
	id string,
	userID string,
) (models.Sighting, error) {
	current, found, err := s.sightings.GetForUser(ctx, id, userID)
	if err != nil {
		return models.Sighting{}, err
	}
	if !found {
		return models.Sighting{}, models.ErrNotFound("sighting not found")
	}

	return models.Sighting{}, &models.StaleError{
		Current: current,
	}
}

// --- helpers ---

func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultPageLimit
	case limit > maxPageLimit:
		return maxPageLimit
	default:
		return limit
	}
}

func distinctBirdIDs(items []models.SightingSync) []string {
	seen := make(map[string]struct{})
	var ids []string
	for _, it := range items {
		if it.BirdID != nil {
			if _, ok := seen[*it.BirdID]; !ok {
				seen[*it.BirdID] = struct{}{}
				ids = append(ids, *it.BirdID)
			}
		}
	}
	return ids
}

// toSighting maps a validated sync item onto the storage model; capture fields are carried through but only persisted on insert by the repository.
func toSighting(userID string, item models.SightingSync) models.Sighting {
	return models.Sighting{
		ID:                      item.ID,
		UserID:                  userID,
		ObservedAt:              item.ObservedAt,
		ObservedAtOffsetMinutes: item.ObservedAtOffsetMinutes,
		ClientUpdatedAt:         item.ClientUpdatedAt,
		BirdID:                  item.BirdID,
		QuickNote:               item.QuickNote,
		Notes:                   item.Notes,
		Latitude:                item.Latitude,
		Longitude:               item.Longitude,
		AccuracyM:               item.AccuracyM,
		Deleted:                 item.Deleted,
	}
}
