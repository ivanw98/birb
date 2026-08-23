package service

import (
	"context"
	"time"

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

// mockFeedRepo records the window the service computed, which is the whole point of the
// clock seam: nothing else can observe it.
type mockFeedRepo struct {
	ListPageFn func(ctx context.Context, authorIDs []string, since, until time.Time, cursor *models.Cursor, limit int) ([]models.FeedItem, error)
	Calls      int
	GotSince   time.Time
	GotUntil   time.Time
	GotAuthors []string
	GotCursor  *models.Cursor
	GotLimit   int
}

func (m *mockFeedRepo) ListPage(ctx context.Context, authorIDs []string, since, until time.Time, cursor *models.Cursor, limit int) ([]models.FeedItem, error) {
	m.Calls++
	m.GotSince, m.GotUntil, m.GotAuthors, m.GotCursor, m.GotLimit = since, until, authorIDs, cursor, limit
	if m.ListPageFn == nil {
		return nil, nil
	}
	return m.ListPageFn(ctx, authorIDs, since, until, cursor, limit)
}

// mockGroupRepo implements the whole interface but only the feed's method is wired; the
// rest panic rather than returning a zero value that could mask a wrong call.
type mockGroupRepo struct {
	CoMemberIDsFn func(ctx context.Context, userID string) ([]string, error)
}

func (m *mockGroupRepo) CoMemberIDs(ctx context.Context, userID string) ([]string, error) {
	return m.CoMemberIDsFn(ctx, userID)
}
func (m *mockGroupRepo) ListForUser(context.Context, string, *string) ([]models.Group, error) {
	panic("unexpected ListForUser")
}
func (m *mockGroupRepo) Create(context.Context, string, string, string, string) error {
	panic("unexpected Create")
}
func (m *mockGroupRepo) FindByJoinCode(context.Context, string) (string, string, bool, error) {
	panic("unexpected FindByJoinCode")
}
func (m *mockGroupRepo) GetOwner(context.Context, string) (string, bool, error) {
	panic("unexpected GetOwner")
}
func (m *mockGroupRepo) AddMember(context.Context, string, string) error {
	panic("unexpected AddMember")
}
func (m *mockGroupRepo) RemoveMember(context.Context, string, string) error {
	panic("unexpected RemoveMember")
}
func (m *mockGroupRepo) Delete(context.Context, string) error { panic("unexpected Delete") }
func (m *mockGroupRepo) IsMember(context.Context, string, string) (bool, error) {
	panic("unexpected IsMember")
}
func (m *mockGroupRepo) CountMembers(context.Context, string) (int, error) {
	panic("unexpected CountMembers")
}
func (m *mockGroupRepo) CountMemberships(context.Context, string) (int, error) {
	panic("unexpected CountMemberships")
}
func (m *mockGroupRepo) CountOwned(context.Context, string) (int, error) {
	panic("unexpected CountOwned")
}

var (
	_ store.FeedRepository     = (*mockFeedRepo)(nil)
	_ store.GroupRepository    = (*mockGroupRepo)(nil)
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
