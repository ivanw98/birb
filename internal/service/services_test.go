package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanw98/birb/internal/models"
)

func TestBirdServiceListReturnsETag(t *testing.T) {
	birds := []models.Bird{{ID: "brd_1", CommonName: "Robin", ScientificName: "Erithacus rubecula"}}
	br := &mockBirdRepo{ListFn: func(_ context.Context) ([]models.Bird, error) { return birds, nil }}
	svc := NewBirds(br)

	got, etag, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, birds, got)
	assert.Equal(t, models.BirdsETag(birds), etag)
	assert.Regexp(t, `^"[0-9a-f]{64}"$`, etag)
}

func TestBirdServiceListError(t *testing.T) {
	br := &mockBirdRepo{ListFn: func(_ context.Context) ([]models.Bird, error) { return nil, sql.ErrConnDone }}
	_, _, err := NewBirds(br).List(context.Background())
	assert.Error(t, err)
}

func TestAccountServiceMe(t *testing.T) {
	ur := &mockUserRepo{GetByAuthIDFn: func(_ context.Context, authID string) (models.User, error) {
		return models.User{ID: "usr_1", AuthID: authID, Email: "a@b.co", Tier: models.TierPremium}, nil
	}}
	me, err := NewAccount(ur).Me(context.Background(), testAuthID)
	require.NoError(t, err)
	assert.Equal(t, "usr_1", me.ID)
	assert.Equal(t, models.TierPremium, me.Tier)
}

func TestAccountServiceMeError(t *testing.T) {
	ur := &mockUserRepo{GetByAuthIDFn: func(_ context.Context, _ string) (models.User, error) {
		return models.User{}, sql.ErrNoRows
	}}
	_, err := NewAccount(ur).Me(context.Background(), testAuthID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
