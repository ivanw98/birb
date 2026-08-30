package service

import (
	"net/http"
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
	maxRecordingPaths = 5
	maxMediaPathLen   = 512
	clockSkewGraceHrs = 24
)

var (
	sightingIDPattern    = regexp.MustCompile(`^sgh_[a-z0-9]{26}$`)
	birdIDPattern        = regexp.MustCompile(`^brd_[a-z0-9]{26}$`)
	photoFilePattern     = `[A-Za-z0-9._-]+\.(jpe?g|png|webp|heic)`
	recordingFilePattern = `[A-Za-z0-9._-]+\.(webm|ogg|m4a|mp4)`
)

// mediaPathRegex validates that a media path is prefixed with the caller's
// auth uid and the sighting's id, and ends with an allowed file pattern.
func mediaPathRegex(authID, sightingID, filePattern string) *regexp.Regexp {
	return regexp.MustCompile(
		`^` + regexp.QuoteMeta(authID) + `/` + regexp.QuoteMeta(sightingID) + `/` + filePattern + `$`,
	)
}

// validateClientUpdatedAt guards the last-write-wins arbiter on BOTH write
// paths (batch and PUT): it must be present and at most 24h in the future.
// Without the future bound, one write from a fast device clock outranks every
// later legitimate edit of the row, with no recovery short of manual SQL.
func (s *Sightings) validateClientUpdatedAt(t time.Time) *models.APIError {
	if t.IsZero() {
		return apiErr(models.CodeValidationFailed, "clientUpdatedAt is required")
	}
	if t.After(s.now().Add(clockSkewGraceHrs * time.Hour)) {
		return apiErr(models.CodeClientTSInFuture, "clientUpdatedAt is more than 24h in the future")
	}
	return nil
}

// validateContentLengths bounds the free-text content fields on both write
// paths, mirroring the DB CHECK constraints so violations surface as 400s
// rather than constraint errors.
func validateContentLengths(quickNote, notes *string) *models.APIError {
	if quickNote != nil && len(*quickNote) > maxQuickNoteLen {
		return apiErr(models.CodeValidationFailed, "quickNote too long")
	}
	if notes != nil && len(*notes) > maxNotesLen {
		return apiErr(models.CodeValidationFailed, "notes too long")
	}
	return nil
}

// validateSyncItem returns an APIError describing the first problem with item, or nil if it is well-formed.
func (s *Sightings) validateSyncItem(item models.SightingSync, birdExists map[string]struct{}) *models.APIError {
	if !sightingIDPattern.MatchString(item.ID) {
		return apiErr(models.CodeValidationFailed, "id must match ^sgh_[a-z0-9]{26}$")
	}
	if item.ObservedAt.After(s.now().Add(clockSkewGraceHrs * time.Hour)) {
		return apiErr(models.CodeObservedInFuture, "observedAt is more than 24h in the future")
	}
	if ae := s.validateClientUpdatedAt(item.ClientUpdatedAt); ae != nil {
		return ae
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
	if ae := validateContentLengths(item.QuickNote, item.Notes); ae != nil {
		return ae
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

// asBadRequest lifts a per-item APIError into the request-level 400 CodedError
// that the PUT path renders, so both write paths share one set of validators.
func asBadRequest(ae *models.APIError) *models.CodedError {
	return models.Coded(http.StatusBadRequest, ae.Code, ae.Message)
}
