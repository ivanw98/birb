package service

import (
	"context"

	"github.com/ivanw98/birb/internal/models"
	"github.com/ivanw98/birb/internal/store"
)

type mockSightingRepo struct {
	UpsertFn func(ctx context.Context, s models.Sighting) (store.UpsertOutcome, error)
	ListFn   func(ctx context.Context, userID string, cursor *models.Cursor, limit int, includeDeleted bool) ([]models.Sighting, error)
	GetFn    func(ctx context.Context, id, userID string) (*models.Sighting, bool, error)
	UpdateFn func(ctx context.Context, id, userID string, upd models.SightingUpdate) (*models.Sighting, bool, error)
}

func (m *mockSightingRepo) Upsert(ctx context.Context, s models.Sighting) (store.UpsertOutcome, error) {
	return m.UpsertFn(ctx, s)
}
func (m *mockSightingRepo) ListByUser(ctx context.Context, userID string, cursor *models.Cursor, limit int, includeDeleted bool) ([]models.Sighting, error) {
	return m.ListFn(ctx, userID, cursor, limit, includeDeleted)
}
func (m *mockSightingRepo) GetForUser(ctx context.Context, id, userID string) (*models.Sighting, bool, error) {
	return m.GetFn(ctx, id, userID)
}
func (m *mockSightingRepo) UpdateContent(ctx context.Context, id, userID string, upd models.SightingUpdate) (*models.Sighting, bool, error) {
	return m.UpdateFn(ctx, id, userID, upd)
}

type mockBirdRepo struct {
	ListFn        func(ctx context.Context) ([]models.Bird, error)
	ExistingIDsFn func(ctx context.Context, ids []string) (map[string]struct{}, error)
}

func (m *mockBirdRepo) List(ctx context.Context) ([]models.Bird, error) { return m.ListFn(ctx) }
func (m *mockBirdRepo) ExistingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	return m.ExistingIDsFn(ctx, ids)
}

type mockUserRepo struct {
	UpsertFn      func(ctx context.Context, u models.User) (models.User, error)
	GetByAuthIDFn func(ctx context.Context, authID string) (models.User, error)
}

func (m *mockUserRepo) Upsert(ctx context.Context, u models.User) (models.User, error) {
	return m.UpsertFn(ctx, u)
}
func (m *mockUserRepo) GetByAuthID(ctx context.Context, authID string) (models.User, error) {
	return m.GetByAuthIDFn(ctx, authID)
}

var (
	_ store.SightingRepository = (*mockSightingRepo)(nil)
	_ store.BirdRepository     = (*mockBirdRepo)(nil)
	_ store.UserRepository     = (*mockUserRepo)(nil)
)

func birdSet(ids ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}
