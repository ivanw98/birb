-- +goose Up
-- +goose StatementBegin

CREATE TABLE groups (
    id            text PRIMARY KEY CHECK (id ~ '^grp_[a-z0-9]{26}$'),
    name          text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    owner_user_id text NOT NULL REFERENCES users (id),
    join_code     text NOT NULL UNIQUE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX groups_owner_idx ON groups (owner_user_id);

CREATE TABLE group_members (
    group_id  text NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    user_id   text NOT NULL REFERENCES users (id),
    joined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

-- The PK leads with group_id; the feed reads the other direction.
CREATE INDEX group_members_user_idx ON group_members (user_id);

-- Deny-by-default for PostgREST's anon key, per 00001_init.sql. Without it every
-- join code in the table is world-readable.
ALTER TABLE groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE group_members ENABLE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
-- +goose StatementEnd
