// Package entitlements resolves a user's tier into a Limits struct and stores it in the request context.
package entitlements

import (
	"context"
	"net/http"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/models"
)

// Limits are the per-request entitlement ceilings, where zero means unlimited.
type Limits struct {
	// MaxBatchItems caps a single batch sync (0 = use the API default of 100).
	MaxBatchItems int
	// MonthlySyncs caps synced sightings per month (0 = unlimited).
	MonthlySyncs int
	// AdvancedBirdID gates the advanced bird identification feature.
	AdvancedBirdID bool
}

// LimitsFor returns the entitlement ceilings for a tier.
func LimitsFor(tier models.Tier) Limits {
	switch tier {
	case models.TierPremium:
		return Limits{AdvancedBirdID: true}
	default:
		return Limits{}
	}
}

type ctxKey int

const limitsKey ctxKey = iota

// WithLimits stores limits in the context.
func WithLimits(ctx context.Context, l Limits) context.Context {
	return context.WithValue(ctx, limitsKey, l)
}

// LimitsFrom retrieves limits, defaulting to the free tier when absent.
func LimitsFrom(ctx context.Context) Limits {
	if l, ok := ctx.Value(limitsKey).(Limits); ok {
		return l
	}
	return LimitsFor(models.TierFree)
}

// Middleware resolves the authenticated user's tier into Limits and adds them to the context; it must be mounted after auth.Authenticator and falls back to free-tier limits when no user is present.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tier := models.TierFree
		if u, ok := auth.UserFrom(r.Context()); ok {
			tier = u.Tier
		}
		ctx := WithLimits(r.Context(), LimitsFor(tier))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
