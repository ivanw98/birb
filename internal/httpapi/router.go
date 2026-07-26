// Package httpapi assembles the chi router that fronts the birb API.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/entitlements"
	"github.com/ivanw98/birb/internal/handler"
)

// NewRouter builds the HTTP handler: base middleware, an unauthenticated /healthz, and the authenticated /api group with h's routes mounted.
func NewRouter(h *handler.Handler, authn *auth.Authenticator) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// No RealIP: it is deprecated (spoofable — it trusts X-Forwarded-For /
	// X-Real-IP unconditionally) and nothing here reads the client IP. If
	// something needs it, use middleware.ClientIPFromXFFTrustedProxies plus
	// middleware.GetClientIP rather than mutating r.RemoteAddr.
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(authn.Middleware)
		r.Use(entitlements.Middleware)
		h.Register(r)
	})
	return r
}
