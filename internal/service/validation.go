package service

import (
	"regexp"
	"time"

	"github.com/ivanw98/birb/internal/models"
)

// Validation limits mirror the API contract (openapi.yaml) and the DB CHECK constraints (00001_init.sql).
const (
	maxBatchItems     = 100
	maxQuickNoteLen   = 280
	maxNotesLen       = 5000
	maxOffsetMinutes  = 840
	maxPhotoPaths     = 10
	clockSkewGraceHrs = 24
)

var (
	sightingIDPattern = regexp.MustCompile(`^sgh_[a-z0-9]{26}$`)
	birdIDPattern     = regexp.MustCompile(`^brd_[a-z0-9]{26}$`)
	photoFilePattern  = `[A-Za-z0-9._-]+\.(jpe?g|png|webp|heic)`
)

// photoPathRegex validates that a photo path is prefixed with the caller's auth uid and the sighting's id.
func photoPathRegex(authID, sightingID string) *regexp.Regexp {
	return regexp.MustCompile(
		`^` + regexp.QuoteMeta(authID) + `/` + regexp.QuoteMeta(sightingID) + `/` + photoFilePattern + `$`,
	)
}

// validateSyncItem returns an APIError describing the first problem with item, or nil if it is well-formed.
func (s *Sightings) validateSyncItem(item models.SightingSync, birdExists map[string]struct{}) *models.APIError {
	if !sightingIDPattern.MatchString(item.ID) {
		return apiErr(models.CodeValidationFailed, "id must match ^sgh_[a-z0-9]{26}$")
	}
	limit := s.now().Add(clockSkewGraceHrs * time.Hour)
	if item.ObservedAt.After(limit) {
		return apiErr(models.CodeObservedInFuture, "observedAt is more than 24h in the future")
	}
	if item.ClientUpdatedAt.After(limit) {
		return apiErr(models.CodeClientTSInFuture, "clientUpdatedAt is more than 24h in the future")
	}
	if item.ObservedAtOffsetMinutes < -maxOffsetMinutes || item.ObservedAtOffsetMinutes > maxOffsetMinutes {
		return apiErr(models.CodeValidationFailed, "observedAtOffsetMinutes out of range")
	}
	if item.Latitude != nil && (*item.Latitude < -90 || *item.Latitude > 90) {
		return apiErr(models.CodeValidationFailed, "latitude out of range")
	}
	if item.Longitude != nil && (*item.Longitude < -180 || *item.Longitude > 180) {
		return apiErr(models.CodeValidationFailed, "longitude out of range")
	}
	if item.AccuracyM != nil && *item.AccuracyM < 0 {
		return apiErr(models.CodeValidationFailed, "accuracyM must be >= 0")
	}
	if item.QuickNote != nil && len(*item.QuickNote) > maxQuickNoteLen {
		return apiErr(models.CodeValidationFailed, "quickNote too long")
	}
	if item.Notes != nil && len(*item.Notes) > maxNotesLen {
		return apiErr(models.CodeValidationFailed, "notes too long")
	}
	if item.BirdID != nil {
		if !birdIDPattern.MatchString(*item.BirdID) {
			return apiErr(models.CodeValidationFailed, "birdId must match ^brd_[a-z0-9]{26}$")
		}
		if _, ok := birdExists[*item.BirdID]; !ok {
			return apiErr(models.CodeUnknownBird, "birdId does not exist: "+*item.BirdID)
		}
	}
	return nil
}

func apiErr(code, msg string) *models.APIError {
	return &models.APIError{Code: code, Message: msg}
}
