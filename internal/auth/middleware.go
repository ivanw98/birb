package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ivanw98/birb/internal/models"
	"github.com/ivanw98/birb/internal/store"
)

type ctxKey int

const userKey ctxKey = iota

// WithUser stores the authenticated user in the context.
func WithUser(ctx context.Context, u models.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFrom retrieves the authenticated user, if any.
func UserFrom(ctx context.Context) (models.User, bool) {
	u, ok := ctx.Value(userKey).(models.User)
	return u, ok
}

// MustUser returns the authenticated user or panics — use only in handlers
// mounted behind Authenticator.Middleware, where presence is guaranteed.
func MustUser(ctx context.Context) models.User {
	u, ok := UserFrom(ctx)
	if !ok {
		panic("auth: no user in context; handler not mounted behind Authenticator")
	}
	return u
}

// Authenticator is middleware that verifies the bearer token and just-in-time provisions the user.
type Authenticator struct {
	verifier TokenVerifier
	users    store.UserRepository
	log      *slog.Logger
	newID    func() string
}

// NewAuthenticator builds the middleware.
func NewAuthenticator(v TokenVerifier, users store.UserRepository, log *slog.Logger) *Authenticator {
	return &Authenticator{verifier: v, users: users, log: log, newID: generateUserID}
}

// Middleware wraps next, rejecting unauthenticated requests with 401.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := bearerToken(r)
		if err != nil {
			a.unauthorized(w, "missing or malformed Authorization header")
			return
		}
		claims, err := a.verifier.Verify(r.Context(), raw)
		if err != nil {
			a.log.DebugContext(r.Context(), "token verification failed", "error", err)
			a.unauthorized(w, "invalid or expired token")
			return
		}

		user, err := a.users.Upsert(r.Context(), claims.ToUser(a.newID()))
		if err != nil {
			a.log.ErrorContext(r.Context(), "user provisioning failed", "error", err)
			writeError(w, http.StatusInternalServerError, models.CodeInternal, "could not provision user")
			return
		}

		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

func bearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", errMissingToken
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", errMissingToken
	}
	token := strings.TrimSpace(h[len(prefix):])
	if token == "" {
		return "", errMissingToken
	}
	return token, nil
}

func (a *Authenticator) unauthorized(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusUnauthorized, models.CodeUnauthorized, msg)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(models.APIError{Code: code, Message: msg})
}

type sentinel string

func (s sentinel) Error() string { return string(s) }

const errMissingToken sentinel = "missing bearer token"
