-- +goose Up
-- +goose StatementBegin

-- No FK to auth.users: these run against vanilla Postgres in CI and local docker.
CREATE TABLE users (
    id           text PRIMARY KEY CHECK (id ~ '^usr_[a-z0-9]{26}$'),
    auth_id      uuid NOT NULL UNIQUE,
    email        text NOT NULL CHECK (char_length(email) <= 320),
    display_name text CHECK (char_length(display_name) <= 200),
    tier         text NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'premium')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Seeded in 00002. ebird_code is nullable: some BOU rarities have no eBird mapping.
CREATE TABLE birds (
    id              text PRIMARY KEY CHECK (id ~ '^brd_[a-z0-9]{26}$'),
    common_name     text NOT NULL,
    scientific_name text NOT NULL,
    ebird_code      text UNIQUE,
    taxonomic_order integer
);

CREATE INDEX birds_common_name_lower_idx ON birds (lower(common_name));

CREATE TABLE sightings (
    id                        text PRIMARY KEY CHECK (id ~ '^sgh_[a-z0-9]{26}$'),
    user_id                   text NOT NULL REFERENCES users (id),
    bird_id                   text REFERENCES birds (id),
    quick_note                text CHECK (char_length(quick_note) <= 280),
    notes                     text CHECK (char_length(notes) <= 5000),
    latitude                  double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude                 double precision CHECK (longitude BETWEEN -180 AND 180),
    accuracy_m                double precision CHECK (accuracy_m >= 0),
    photo_paths               text[] NOT NULL DEFAULT '{}'
                                  CHECK (cardinality(photo_paths) <= 10
                                     AND octet_length(array_to_string(photo_paths, '/')) <= 5120),
    observed_at               timestamptz NOT NULL,
    observed_at_offset_minutes integer NOT NULL DEFAULT 0
                                  CHECK (observed_at_offset_minutes BETWEEN -840 AND 840),
    client_updated_at         timestamptz NOT NULL,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    deleted_at                timestamptz
);

-- Serves the keyset walk: ORDER BY observed_at DESC, id DESC.
CREATE INDEX sightings_user_feed_idx
    ON sightings (user_id, observed_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX sightings_bird_id_idx ON sightings (bird_id) WHERE bird_id IS NOT NULL;

-- RLS on with zero policies: deny-by-default for PostgREST's anon key, which ships
-- in the frontend bundle. The API connects as the table owner and bypasses it.
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE birds ENABLE ROW LEVEL SECURITY;
ALTER TABLE sightings ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_tables
        WHERE schemaname = 'public' AND tablename = 'goose_db_version'
    ) THEN
        EXECUTE 'ALTER TABLE public.goose_db_version ENABLE ROW LEVEL SECURITY';
    END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sightings;
DROP TABLE IF EXISTS birds;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
