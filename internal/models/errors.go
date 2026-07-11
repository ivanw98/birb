package models

import (
	"errors"
	"fmt"
	"net/http"
)

// Stable error slugs — the wire `code` values clients branch on; do not rename casually.
const (
	CodeUnknownBird      = "unknown_bird_id"
	CodeIDConflict       = "id_conflict"
	CodeObservedInFuture = "observed_at_in_future"
	CodeClientTSInFuture = "client_updated_at_in_future"
	CodeValidationFailed = "validation_failed"
	CodeNotFound         = "not_found"
	CodeStaleUpdate      = "stale_update"
	CodeBatchTooLarge    = "batch_too_large"
	CodeInvalidPhotoPath = "invalid_photo_path"
	CodeBadRequest       = "bad_request"
	CodeUnauthorized     = "unauthorized"
	CodeInternal         = "internal_error"
)

// CodedError is a request-level error that carries the HTTP status and wire code the handler should render.
type CodedError struct {
	HTTPStatus int
	Code       string
	Message    string
}

func (e *CodedError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// APIError projects the coded error onto the wire envelope.
func (e *CodedError) APIError() APIError {
	return APIError{Code: e.Code, Message: e.Message}
}

// Coded builds a CodedError.
func Coded(status int, code, message string) *CodedError {
	return &CodedError{HTTPStatus: status, Code: code, Message: message}
}

// Common request-level constructors.

func ErrNotFound(message string) *CodedError {
	return Coded(http.StatusNotFound, CodeNotFound, message)
}

func ErrValidation(message string) *CodedError {
	return Coded(http.StatusBadRequest, CodeValidationFailed, message)
}

func ErrBadRequest(message string) *CodedError {
	return Coded(http.StatusBadRequest, CodeBadRequest, message)
}

func ErrUnknownBird(message string) *CodedError {
	return Coded(http.StatusBadRequest, CodeUnknownBird, message)
}

func ErrInvalidPhotoPath(message string) *CodedError {
	return Coded(http.StatusBadRequest, CodeInvalidPhotoPath, message)
}

func ErrBatchTooLarge(message string) *CodedError {
	return Coded(http.StatusBadRequest, CodeBatchTooLarge, message)
}

func ErrUnauthorized(message string) *CodedError {
	return Coded(http.StatusUnauthorized, CodeUnauthorized, message)
}

func ErrInternal(message string) *CodedError {
	return Coded(http.StatusInternalServerError, CodeInternal, message)
}

// AsCoded extracts a *CodedError from err, or returns a generic internal error
// so handlers always have a status + code to render without leaking details.
func AsCoded(err error) *CodedError {
	var ce *CodedError
	if errors.As(err, &ce) {
		return ce
	}
	return ErrInternal("an unexpected error occurred")
}

// StaleError signals a last-write-wins conflict on an interactive update, carrying the current server state for a 409 response.
type StaleError struct {
	Current *Sighting
}

func (e *StaleError) Error() string { return CodeStaleUpdate }
