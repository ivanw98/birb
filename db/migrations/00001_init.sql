-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- users
--
-- id is a server-generated prefixed ULID (`usr_` + 26-char lowercase ULID).
-- auth_id is the Supabase auth.users UUID taken from the verified JWT `sub`
-- claim; it is the natural key the API upserts on during just-in-time
-- provisioning. We deliberately do NOT add a foreign key to auth.users so that
-- these migrations run unchanged against a vanilla Postgres (CI, local docker)
-- with no Supabase-managed `auth` schema present.
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id           text PRIMARY KEY CHECK (id ~ '^usr_[a-z0-9]{26}$'),
    auth_id      uuid NOT NULL UNIQUE,
    email        text NOT NULL,
    display_name text,
    tier         text NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'premium')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- birds
--
-- The UK species reference list, seeded in 00002 with frozen `brd_` ids. Small
-- and effectively static. ebird_code is nullable because a handful of BOU
-- rarities do not map cleanly onto the eBird/Clements taxonomy.
-- ---------------------------------------------------------------------------
CREATE TABLE birds (
    id              text PRIMARY KEY CHECK (id ~ '^brd_[a-z0-9]{26}$'),
    common_name     text NOT NULL,
    scientific_name text NOT NULL,
    ebird_code      text UNIQUE,
    taxonomic_order integer
);

-- Case-insensitive lookups by common name (admin/reporting; the client-side
-- typeahead runs against the bundled copy of this list).
CREATE INDEX birds_common_name_lower_idx ON birds (lower(common_name));

-- ---------------------------------------------------------------------------
-- sightings
--
-- id is client-generated (`sgh_` + ULID) and is the idempotency key for offline
-- batch sync. The server stores three timestamps with distinct jobs:
--   observed_at         device wall-clock instant of the observation (domain)
--   client_updated_at   device time of the last local edit; the last-write-wins
--                       guard for batch upserts
--   created_at/updated_at  server-authoritative receipt and last-write times
-- observed_at_offset_minutes preserves the local UTC offset at capture so the
-- historical wall-clock time survives regardless of where the row is viewed.
-- deleted_at is a soft-delete tombstone (no delete endpoint yet, but multi-
-- device sync will need tombstones and the column is free now).
-- There is intentionally no `status` column: pending/synced is client-only
-- state, and any row the server holds is by definition synced.
-- ---------------------------------------------------------------------------
CREATE TABLE sightings (
    id                        text PRIMARY KEY CHECK (id ~ '^sgh_[a-z0-9]{26}$'),
    user_id                   text NOT NULL REFERENCES users (id),
    bird_id                   text REFERENCES birds (id),
    quick_note                text,
    notes                     text,
    latitude                  double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude                 double precision CHECK (longitude BETWEEN -180 AND 180),
    accuracy_m                double precision CHECK (accuracy_m >= 0),
    photo_paths               text[] NOT NULL DEFAULT '{}' CHECK (cardinality(photo_paths) <= 10),
    observed_at               timestamptz NOT NULL,
    observed_at_offset_minutes integer NOT NULL DEFAULT 0
                                  CHECK (observed_at_offset_minutes BETWEEN -840 AND 840),
    client_updated_at         timestamptz NOT NULL,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    deleted_at                timestamptz
);

-- The one index the read path needs: keyset pagination of a user's feed,
-- newest first, excluding tombstones. Matches ORDER BY observed_at DESC, id DESC
-- with the WHERE (observed_at, id) < (cursor) predicate.
CREATE INDEX sightings_user_feed_idx
    ON sightings (user_id, observed_at DESC, id DESC)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Row Level Security: enable on every public table with NO policies.
--
-- Supabase exposes the `public` schema through PostgREST using the anon key
-- that ships inside the frontend bundle. Enabling RLS with zero policies is
-- deny-by-default for the anon/authenticated PostgREST roles, so no client can
-- read or write these tables directly. The Go API connects as the table-owning
-- role (which bypasses RLS) and remains the single place authorization lives.
-- supabase-js is used ONLY for auth and Storage, never for table access.
-- ---------------------------------------------------------------------------
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE birds ENABLE ROW LEVEL SECURITY;
ALTER TABLE sightings ENABLE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sightings;
DROP TABLE IF EXISTS birds;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
