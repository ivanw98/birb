// Package service is the business-logic layer between the handler and store packages.
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/ivanw98/birb/internal/models"
	"github.com/ivanw98/birb/internal/store"
)

// SightingService orchestrates the sighting flows.
type SightingService interface {
	// BatchSync validates and applies a batch of sightings, returning a per-item result and erroring only on whole-request failures.
	BatchSync(ctx context.Context, user models.User, req models.BatchSyncRequest) (models.BatchSyncResponse, error)
	// List returns a page of the user's sightings, newest first; includeDeleted
	// adds tombstones (marked deleted) so multi-device clients can converge.
	List(ctx context.Context, user models.User, limit int, cursor string, includeDeleted bool) (models.SightingPage, error)
	// Update enriches a sighting, returning *models.StaleError on a stale write or a not-found CodedError when the row is absent or not the caller's.
	Update(ctx context.Context, user models.User, id string, upd models.SightingUpdate) (models.Sighting, error)
}

// BirdService serves the reference list.
type BirdService interface {
	// List returns all birds and a strong ETag for conditional requests.
	List(ctx context.Context) ([]models.Bird, string, error)
}

// GroupService orchestrates group membership.
type GroupService interface {
	// List returns every group the caller belongs to, with full membership.
	List(ctx context.Context, user models.User) ([]models.Group, error)
	// Create mints a group owned by the caller. Not idempotent.
	Create(ctx context.Context, user models.User, req models.CreateGroupRequest) (models.Group, error)
	// Join adds the caller to the group holding the code; re-joining returns it unchanged.
	Join(ctx context.Context, user models.User, req models.JoinGroupRequest) (models.Group, error)
	// Leave drops the caller's membership; idempotent, and refused for owners.
	Leave(ctx context.Context, user models.User, groupID string) error
	// Delete removes a group; owner only, idempotent.
	Delete(ctx context.Context, user models.User, groupID string) error
	// RemoveMember evicts another member; owner only, idempotent.
	RemoveMember(ctx context.Context, user models.User, groupID, memberID string) error
}

// AccountService serves the caller's profile.
type AccountService interface {
	// Me returns the profile for a Supabase auth id.
	Me(ctx context.Context, authID string) (models.Me, error)
}

// Sightings is the concrete SightingService.
type Sightings struct {
	sightings store.SightingRepository
	birds     store.BirdRepository
	log       *slog.Logger
	// now is injectable so the clock-skew rules are deterministically testable.
	now func() time.Time
}

var _ SightingService = (*Sightings)(nil)

// NewSightings builds the sighting service.
func NewSightings(sightings store.SightingRepository, birds store.BirdRepository, log *slog.Logger) *Sightings {
	return &Sightings{sightings: sightings, birds: birds, log: log, now: time.Now}
}

// Birds is the concrete BirdService.
type Birds struct {
	repo store.BirdRepository
}

var _ BirdService = (*Birds)(nil)

// NewBirds builds the bird service.
func NewBirds(repo store.BirdRepository) *Birds { return &Birds{repo: repo} }

// List returns all birds plus their collective ETag.
func (b *Birds) List(ctx context.Context) ([]models.Bird, string, error) {
	birds, err := b.repo.List(ctx)
	if err != nil {
		return nil, "", err
	}
	return birds, models.BirdsETag(birds), nil
}

// Account is the concrete AccountService.
type Account struct {
	repo store.UserRepository
}

var _ AccountService = (*Account)(nil)

// NewAccount builds the account service.
func NewAccount(repo store.UserRepository) *Account { return &Account{repo: repo} }

// Me returns the profile for a Supabase auth id.
func (a *Account) Me(ctx context.Context, authID string) (models.Me, error) {
	u, err := a.repo.GetByAuthID(ctx, authID)
	if err != nil {
		return models.Me{}, err
	}
	return u.ToMe(), nil
}
