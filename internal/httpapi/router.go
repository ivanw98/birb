// Package httpapi assembles the chi router that fronts the birb API.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/entitlements"
	"github.com/ivanw98/birb/internal/handler"
	"github.com/ivanw98/birb/internal/models"
)

// NewRouter builds the HTTP handler: base middleware, an unauthenticated /healthz, the authenticated
// /api group with h's routes mounted, and the built SPA on everything else when staticDir is set.
func NewRouter(h *handler.Handler, authn *auth.Authenticator, staticDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	// Neither host's proxy compresses for us and the main bundle is ~1.5 MB.
	r.Use(middleware.Compress(5))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(authn.Middleware)
		r.Use(entitlements.Middleware)
		h.Register(r)
		// Without this chi propagates the SPA fallback below into this sub-router,
		// and an /api typo returns index.html with a 200.
		r.NotFound(apiNotFound)
	})

	// Must stay after the /api route, which chi only skips because it now has its own NotFound.
	if staticDir != "" {
		r.NotFound(newSPAHandler(staticDir))
	}
	return r
}

// apiNotFound renders the wire error envelope for an unknown /api path.
func apiNotFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(models.APIError{
		Code:    models.CodeNotFound,
		Message: "no such endpoint",
	})
}
