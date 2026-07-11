// Package handler is the HTTP layer: it maps requests onto the service interfaces, renders the coded-error taxonomy to status codes, and owns the chi routing.
// Handlers read the authenticated user from the request context, populated by auth.Authenticator.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/models"
	"github.com/ivanw98/birb/internal/service"
)

// Handler bundles the service dependencies for the HTTP endpoints.
type Handler struct {
	sightings service.SightingService
	birds     service.BirdService
	account   service.AccountService
	log       *slog.Logger
}

// New builds the handler.
func New(s service.SightingService, b service.BirdService, a service.AccountService, log *slog.Logger) *Handler {
	return &Handler{sightings: s, birds: b, account: a, log: log}
}

// Register mounts the API endpoints on r; callers must apply auth/entitlements middleware to the router group first.
func (h *Handler) Register(r chi.Router) {
	r.Post("/sightings/batch", h.BatchSync)
	r.Get("/sightings", h.List)
	r.Put("/sightings/{id}", h.Update)
	r.Get("/birds", h.Birds)
	r.Get("/me", h.Me)
}

// BatchSync handles POST /api/sightings/batch.
func (h *Handler) BatchSync(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	var req models.BatchSyncRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.renderError(w, r, decodeError(err))
		return
	}
	resp, err := h.sightings.BatchSync(r.Context(), user, req)
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// List handles GET /api/sightings.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			h.renderError(w, r, models.ErrValidation("limit must be an integer"))
			return
		}
		limit = n
	}

	page, err := h.sightings.List(r.Context(), user, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// Update handles PUT /api/sightings/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	id := chi.URLParam(r, "id")

	var upd models.SightingUpdate
	if err := decodeJSON(w, r, &upd); err != nil {
		h.renderError(w, r, decodeError(err))
		return
	}
	if upd.PhotoPaths == nil {
		upd.PhotoPaths = []string{}
	}

	sighting, err := h.sightings.Update(r.Context(), user, id, upd)
	if err != nil {
		var stale *models.StaleError
		if errors.As(err, &stale) {
			// 409 with the current server state so the UI can reconcile.
			writeJSON(w, http.StatusConflict, staleBody{Code: models.CodeStaleUpdate, Current: stale.Current})
			return
		}
		h.renderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sighting)
}

// staleBody mirrors the StaleUpdate schema in the contract.
type staleBody struct {
	Code    string           `json:"code"`
	Current *models.Sighting `json:"current"`
}

// Birds handles GET /api/birds with ETag conditional-request support.
func (h *Handler) Birds(w http.ResponseWriter, r *http.Request) {
	birds, etag, err := h.birds.List(r.Context())
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if ifNoneMatch(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, birds)
}

// Me handles GET /api/me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	me, err := h.account.Me(r.Context(), user.AuthID)
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, me)
}

// --- helpers ---

// maxBodyBytes caps request bodies before decoding. The largest legal payload
// (a 100-item batch with every text field at its maximum) is under 600KB, so
// 1MiB rejects abuse without ever touching a valid request.
const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dst)
}

// decodeError distinguishes an oversized body (413) from malformed JSON (400).
func decodeError(err error) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return models.Coded(http.StatusRequestEntityTooLarge, models.CodeBadRequest, "request body too large")
	}
	return models.ErrBadRequest("malformed JSON body")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (h *Handler) renderError(w http.ResponseWriter, r *http.Request, err error) {
	ce := models.AsCoded(err)
	if ce.HTTPStatus >= http.StatusInternalServerError {
		h.log.ErrorContext(r.Context(), "request failed", "error", err, "code", ce.Code)
	}
	writeJSON(w, ce.HTTPStatus, ce.APIError())
}

// ifNoneMatch reports whether the current strong etag satisfies the client's
// If-None-Match header (a comma-separated list, possibly weak or "*").
func ifNoneMatch(header, current string) bool {
	if header == "" {
		return false
	}
	if strings.TrimSpace(header) == "*" {
		return true
	}
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		tag = strings.TrimPrefix(tag, "W/")
		if tag == current {
			return true
		}
	}
	return false
}
