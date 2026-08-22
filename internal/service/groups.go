package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ivanw98/birb/internal/models"
	"github.com/ivanw98/birb/internal/store"
)

const (
	maxGroupNameLen = 100
	maxJoinCodeLen  = 32
	joinCodeLen     = 8

	maxMembersPerGroup    = 25
	maxMembershipsPerUser = 10
	maxOwnedGroups        = 5

	// createAttempts bounds the retry when a minted join code collides.
	createAttempts = 5
)

// joinCodeAlphabet is Crockford base32 without the digits 0 and 1: I, L and O go for
// visual confusion, U to keep a random code from spelling something obscene.
const joinCodeAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

// Groups implements GroupService.
type Groups struct {
	groups  store.GroupRepository
	limiter *JoinLimiter
}

var _ GroupService = (*Groups)(nil)

func NewGroups(groups store.GroupRepository, limiter *JoinLimiter) *Groups {
	return &Groups{groups: groups, limiter: limiter}
}

// List returns every group the caller belongs to.
func (s *Groups) List(ctx context.Context, user models.User) ([]models.Group, error) {
	groups, err := s.groups.ListForUser(ctx, user.ID, nil)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []models.Group{}
	}
	return groups, nil
}

// Create mints a group owned by the caller, who becomes its first member.
func (s *Groups) Create(ctx context.Context, user models.User, req models.CreateGroupRequest) (models.Group, error) {
	name := strings.TrimSpace(req.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxGroupNameLen {
		return models.Group{}, models.ErrValidation(fmt.Sprintf("name must be 1..%d characters", maxGroupNameLen))
	}

	owned, err := s.groups.CountOwned(ctx, user.ID)
	if err != nil {
		return models.Group{}, err
	}
	if owned >= maxOwnedGroups {
		return models.Group{}, models.ErrGroupLimitReached(fmt.Sprintf("you can own at most %d groups", maxOwnedGroups))
	}

	for range createAttempts {
		id := models.NewID(models.PrefixGroup).String()
		code, err := newJoinCode()
		if err != nil {
			return models.Group{}, err
		}

		err = s.groups.Create(ctx, id, name, user.ID, code)
		if errors.Is(err, store.ErrJoinCodeTaken) {
			continue
		}
		if err != nil {
			return models.Group{}, err
		}
		return s.one(ctx, user, id)
	}
	return models.Group{}, models.ErrInternal("could not allocate a unique join code")
}

// Join adds the caller to the group holding the given code.
func (s *Groups) Join(ctx context.Context, user models.User, req models.JoinGroupRequest) (models.Group, error) {
	if utf8.RuneCountInString(req.Code) > maxJoinCodeLen {
		return models.Group{}, models.ErrValidation("code is too long")
	}
	if s.limiter.blocked(user.ID) {
		return models.Group{}, models.ErrJoinRateLimited("too many failed attempts; wait a while and try again")
	}

	code, ok := normaliseJoinCode(req.Code)
	if !ok {
		s.limiter.fail(user.ID)
		return models.Group{}, models.ErrUnknownJoinCode("that code did not match a group")
	}

	groupID, _, found, err := s.groups.FindByJoinCode(ctx, code)
	if err != nil {
		return models.Group{}, err
	}
	if !found {
		s.limiter.fail(user.ID)
		return models.Group{}, models.ErrUnknownJoinCode("that code did not match a group")
	}

	// Membership is checked before the caps: a re-join must succeed even where a
	// newcomer would be refused, or a retry after a dropped connection dead-ends.
	member, err := s.groups.IsMember(ctx, groupID, user.ID)
	if err != nil {
		return models.Group{}, err
	}
	if member {
		return s.one(ctx, user, groupID)
	}

	members, err := s.groups.CountMembers(ctx, groupID)
	if err != nil {
		return models.Group{}, err
	}
	if members >= maxMembersPerGroup {
		return models.Group{}, models.ErrGroupFull(fmt.Sprintf("that group already has %d members", maxMembersPerGroup))
	}

	memberships, err := s.groups.CountMemberships(ctx, user.ID)
	if err != nil {
		return models.Group{}, err
	}
	if memberships >= maxMembershipsPerUser {
		return models.Group{}, models.ErrGroupLimitReached(fmt.Sprintf("you can belong to at most %d groups", maxMembershipsPerUser))
	}

	if err := s.groups.AddMember(ctx, groupID, user.ID); err != nil {
		return models.Group{}, err
	}
	return s.one(ctx, user, groupID)
}

// Leave drops the caller's membership. Idempotent; owners are refused.
func (s *Groups) Leave(ctx context.Context, user models.User, groupID string) error {
	if !models.ValidID(models.PrefixGroup, groupID) {
		return models.ErrValidation("group id is malformed")
	}

	owner, found, err := s.groups.GetOwner(ctx, groupID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if owner == user.ID {
		return models.ErrOwnerCannotLeave("owners delete a group rather than leaving it")
	}
	return s.groups.RemoveMember(ctx, groupID, user.ID)
}

// Delete removes a group. Owner only, idempotent.
func (s *Groups) Delete(ctx context.Context, user models.User, groupID string) error {
	if !models.ValidID(models.PrefixGroup, groupID) {
		return models.ErrValidation("group id is malformed")
	}

	owner, found, err := s.groups.GetOwner(ctx, groupID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if owner != user.ID {
		return models.ErrNotGroupOwner("only the group's owner can delete it")
	}
	return s.groups.Delete(ctx, groupID)
}

// RemoveMember evicts someone else. Owner only, idempotent.
func (s *Groups) RemoveMember(ctx context.Context, user models.User, groupID, memberID string) error {
	if !models.ValidID(models.PrefixGroup, groupID) {
		return models.ErrValidation("group id is malformed")
	}
	if !models.ValidID(models.PrefixUser, memberID) {
		return models.ErrValidation("member id is malformed")
	}

	owner, found, err := s.groups.GetOwner(ctx, groupID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	// Ownership is checked before the target is resolved, so a non-owner never learns
	// whether the person they named is a member.
	if owner != user.ID {
		return models.ErrNotGroupOwner("only the group's owner can remove members")
	}
	// Evicting the owner would leave owner_user_id pointing at a non-member: the group
	// would vanish from its owner's list while still existing, and become undeletable.
	if memberID == owner {
		return models.ErrOwnerCannotLeave("the owner cannot be removed; delete the group instead")
	}
	return s.groups.RemoveMember(ctx, groupID, memberID)
}

// one re-reads a single group so create and join return the same shape as list.
func (s *Groups) one(ctx context.Context, user models.User, groupID string) (models.Group, error) {
	groups, err := s.groups.ListForUser(ctx, user.ID, &groupID)
	if err != nil {
		return models.Group{}, err
	}
	if len(groups) == 0 {
		return models.Group{}, models.ErrNotFound("group not found")
	}
	return groups[0], nil
}

// normaliseJoinCode uppercases and strips separators.
func normaliseJoinCode(raw string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(raw) {
		switch {
		case r == ' ' || r == '-' || r == '_':
			continue
		case strings.ContainsRune(joinCodeAlphabet, r):
			b.WriteRune(r)
		default:
			return "", false
		}
	}
	code := b.String()
	return code, len(code) == joinCodeLen
}

// newJoinCode draws from the alphabet without modulo bias.
func newJoinCode() (string, error) {
	out := make([]byte, joinCodeLen)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(joinCodeAlphabet))))
		if err != nil {
			return "", fmt.Errorf("generate join code: %w", err)
		}
		out[i] = joinCodeAlphabet[n.Int64()]
	}
	return string(out), nil
}

// JoinLimiter throttles failed join attempts per user, so a leaked-code guess cannot be
// brute-forced. In-memory and therefore per-process: several API instances would
// multiply the effective allowance, acceptable on a single-machine deploy.
type JoinLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
	now      func() time.Time
	sweepIn  int
}

const (
	defaultJoinFailureLimit  = 20
	defaultJoinFailureWindow = time.Hour
	// sweepEvery bounds map growth: pruning only the key being touched leaves idle
	// users' slices resident forever.
	sweepEvery = 256
)

// NewJoinLimiter builds a limiter; zero max or window take the defaults. The limit is a
// parameter so tests can trip it without issuing twenty-odd requests.
func NewJoinLimiter(max int, window time.Duration) *JoinLimiter {
	if max <= 0 {
		max = defaultJoinFailureLimit
	}
	if window <= 0 {
		window = defaultJoinFailureWindow
	}
	return &JoinLimiter{
		attempts: map[string][]time.Time{},
		max:      max,
		window:   window,
		now:      time.Now,
		sweepIn:  sweepEvery,
	}
}

func (l *JoinLimiter) blocked(userID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(userID, l.now())) >= l.max
}

func (l *JoinLimiter) fail(userID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	live := l.prune(userID, now)
	// Cap the slice: without this a caller already over the limit keeps appending for
	// the rest of the window.
	if len(live) < l.max {
		l.attempts[userID] = append(live, now)
	}

	l.sweepIn--
	if l.sweepIn <= 0 {
		l.sweepIn = sweepEvery
		l.sweep(now)
	}
}

// prune drops attempts that have aged out and stores what is left. Caller holds the lock.
func (l *JoinLimiter) prune(userID string, now time.Time) []time.Time {
	times := l.attempts[userID]
	if len(times) == 0 {
		return nil
	}

	cutoff := now.Add(-l.window)
	kept := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.attempts, userID)
		return nil
	}
	l.attempts[userID] = kept
	return kept
}

// sweep drops users whose attempts have all aged out. Caller holds the lock.
func (l *JoinLimiter) sweep(now time.Time) {
	cutoff := now.Add(-l.window)
	for user, times := range l.attempts {
		if len(times) == 0 || !times[len(times)-1].After(cutoff) {
			delete(l.attempts, user)
		}
	}
}
