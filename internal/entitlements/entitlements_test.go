package entitlements

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/models"
)

func TestLimitsForTiers(t *testing.T) {
	free := LimitsFor(models.TierFree)
	assert.False(t, free.AdvancedBirdID)
	assert.Zero(t, free.MonthlySyncs, "unlimited today")

	premium := LimitsFor(models.TierPremium)
	assert.True(t, premium.AdvancedBirdID)
}

func TestLimitsFromDefaultsToFree(t *testing.T) {
	l := LimitsFrom(context.Background())
	assert.Equal(t, LimitsFor(models.TierFree), l)
}

func TestMiddlewareResolvesTierFromUser(t *testing.T) {
	var got Limits
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = LimitsFrom(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := auth.WithUser(req.Context(), models.User{ID: "usr_1", Tier: models.TierPremium})
	req = req.WithContext(ctx)

	Middleware(next).ServeHTTP(httptest.NewRecorder(), req)
	assert.True(t, got.AdvancedBirdID, "premium user gets premium limits")
}

func TestMiddlewareFallsBackToFreeWithoutUser(t *testing.T) {
	var got Limits
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = LimitsFrom(r.Context())
	})
	Middleware(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, LimitsFor(models.TierFree), got)
}
