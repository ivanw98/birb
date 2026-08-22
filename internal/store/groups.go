package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/ivanw98/birb/internal/models"
)

// GroupStore is the Postgres-backed GroupRepository.
type GroupStore struct {
	db *sqlx.DB
}

func NewGroupStore(db *sqlx.DB) *GroupStore {
	return &GroupStore{db: db}
}

var _ GroupRepository = (*GroupStore)(nil)

const (
	groupsTable       = "groups"
	groupMembersTable = "group_members"
)

// groupMemberRow is one row of the flat group×member join, denormalised so a single
// query yields whole groups.
type groupMemberRow struct {
	GroupID       string  `db:"group_id"`
	Name          string  `db:"name"`
	JoinCode      string  `db:"join_code"`
	OwnerUserID   string  `db:"owner_user_id"`
	MemberID      string  `db:"member_id"`
	MemberName    *string `db:"member_name"`
	MemberIsOwner bool    `db:"member_is_owner"`
}

// listQuery is one statement rather than "my groups, then their members": with no
// transaction, a group deleted between two queries would come back with an empty
// member list, contradicting the invariant that the owner is always a member.
//
// Every ORDER BY term is load-bearing. joined_at defaults to now(), which is
// transaction time, so members inserted by one statement tie; member_id breaks it.
const listQuery = `
SELECT g.id                          AS group_id,
       g.name,
       g.join_code,
       g.owner_user_id,
       m.user_id                     AS member_id,
       u.display_name                AS member_name,
       (m.user_id = g.owner_user_id) AS member_is_owner
FROM group_members me
JOIN groups g ON g.id = me.group_id
JOIN group_members m ON m.group_id = g.id
JOIN users u ON u.id = m.user_id
WHERE me.user_id = $1
  AND ($2::text IS NULL OR g.id = $2)
ORDER BY g.created_at ASC, g.id ASC,
         (m.user_id = g.owner_user_id) DESC,
         m.joined_at ASC,
         m.user_id ASC`

// ListForUser returns every group the user belongs to, or just one when groupID is set.
func (s *GroupStore) ListForUser(ctx context.Context, userID string, groupID *string) ([]models.Group, error) {
	var rows []groupMemberRow
	if err := s.db.SelectContext(ctx, &rows, listQuery, userID, groupID); err != nil {
		return nil, err
	}

	out := make([]models.Group, 0, len(rows))
	byID := make(map[string]int, len(rows))
	for _, r := range rows {
		i, ok := byID[r.GroupID]
		if !ok {
			i = len(out)
			byID[r.GroupID] = i
			out = append(out, models.Group{
				ID:       r.GroupID,
				Name:     r.Name,
				JoinCode: r.JoinCode,
				IsOwner:  r.OwnerUserID == userID,
				Members:  []models.GroupMember{},
			})
		}
		out[i].Members = append(out[i].Members, models.GroupMember{
			ID:      r.MemberID,
			Name:    r.MemberName,
			IsOwner: r.MemberIsOwner,
		})
	}
	return out, nil
}

// createQuery inserts the group and its owner's membership in one statement. The repo
// has no transactions; a data-modifying CTE runs to completion regardless of whether
// the outer query reads it, and the whole statement is atomic. m reads g's RETURNING
// tuplestore, not the table, which is the documented way to chain two inserts.
const createQuery = `
WITH g AS (
    INSERT INTO groups (id, name, owner_user_id, join_code)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (join_code) DO NOTHING
    RETURNING id
), m AS (
    INSERT INTO group_members (group_id, user_id) SELECT id, $3 FROM g
)
SELECT id FROM g`

// ErrJoinCodeTaken reports that the minted code collided; the caller retries with a new one.
var ErrJoinCodeTaken = errors.New("join code already in use")

// Create inserts a group owned by ownerUserID, returning ErrJoinCodeTaken on collision.
func (s *GroupStore) Create(ctx context.Context, id, name, ownerUserID, joinCode string) error {
	var out string
	err := s.db.QueryRowxContext(ctx, createQuery, id, name, ownerUserID, joinCode).Scan(&out)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrJoinCodeTaken
	}
	return err
}

// FindByJoinCode returns the group id and owner for a canonical join code.
func (s *GroupStore) FindByJoinCode(ctx context.Context, joinCode string) (id, ownerUserID string, found bool, err error) {
	query, args, err := builder.
		Select("id", "owner_user_id").
		From(groupsTable).
		Where(sq.Eq{"join_code": joinCode}).
		ToSql()
	if err != nil {
		return "", "", false, fmt.Errorf("build groups find by join code: %w", err)
	}

	err = s.db.QueryRowxContext(ctx, query, args...).Scan(&id, &ownerUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return id, ownerUserID, true, nil
}

// GetOwner returns the group's owner, or found=false if there is no such group.
func (s *GroupStore) GetOwner(ctx context.Context, groupID string) (ownerUserID string, found bool, err error) {
	query, args, err := builder.
		Select("owner_user_id").
		From(groupsTable).
		Where(sq.Eq{"id": groupID}).
		ToSql()
	if err != nil {
		return "", false, fmt.Errorf("build groups get owner: %w", err)
	}

	err = s.db.QueryRowxContext(ctx, query, args...).Scan(&ownerUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return ownerUserID, true, nil
}

// AddMember records a membership; a replay is a no-op on the composite primary key.
func (s *GroupStore) AddMember(ctx context.Context, groupID, userID string) error {
	query, args, err := builder.
		Insert(groupMembersTable).
		Columns("group_id", "user_id").
		Values(groupID, userID).
		Suffix("ON CONFLICT (group_id, user_id) DO NOTHING").
		ToSql()
	if err != nil {
		return fmt.Errorf("build group member insert: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query, args...)
	return err
}

// RemoveMember drops a membership; removing one that is not there is a no-op.
func (s *GroupStore) RemoveMember(ctx context.Context, groupID, userID string) error {
	query, args, err := builder.
		Delete(groupMembersTable).
		Where(sq.Eq{"group_id": groupID, "user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build group member delete: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query, args...)
	return err
}

// Delete removes a group; memberships go with it by cascade.
func (s *GroupStore) Delete(ctx context.Context, groupID string) error {
	query, args, err := builder.
		Delete(groupsTable).
		Where(sq.Eq{"id": groupID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build group delete: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query, args...)
	return err
}

// IsMember reports whether the user already belongs to the group.
func (s *GroupStore) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	query, args, err := builder.
		Select("1").
		From(groupMembersTable).
		Where(sq.Eq{"group_id": groupID, "user_id": userID}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build group membership check: %w", err)
	}

	var one int
	err = s.db.QueryRowxContext(ctx, query, args...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CountMembers returns the group's current size, for the members-per-group cap.
func (s *GroupStore) CountMembers(ctx context.Context, groupID string) (int, error) {
	return s.count(ctx, groupMembersTable, sq.Eq{"group_id": groupID}, "count group members")
}

// CountMemberships returns how many groups the user belongs to, for the memberships-per-user cap.
func (s *GroupStore) CountMemberships(ctx context.Context, userID string) (int, error) {
	return s.count(ctx, groupMembersTable, sq.Eq{"user_id": userID}, "count user memberships")
}

// CountOwned returns how many groups the user owns, for the owned-groups cap.
func (s *GroupStore) CountOwned(ctx context.Context, userID string) (int, error) {
	return s.count(ctx, groupsTable, sq.Eq{"owner_user_id": userID}, "count owned groups")
}

func (s *GroupStore) count(ctx context.Context, table string, where sq.Eq, what string) (int, error) {
	query, args, err := builder.Select("count(*)").From(table).Where(where).ToSql()
	if err != nil {
		return 0, fmt.Errorf("build %s: %w", what, err)
	}

	var n int
	err = s.db.QueryRowxContext(ctx, query, args...).Scan(&n)
	return n, err
}
