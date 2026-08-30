-- +goose Up
-- +goose StatementBegin

-- Bucket-relative Supabase Storage paths for audio recordings (max 5).
ALTER TABLE sightings
    ADD COLUMN recording_paths text[] NOT NULL DEFAULT '{}'
        CHECK (cardinality(recording_paths) <= 5
           AND octet_length(array_to_string(recording_paths, '/')) <= 2564);

-- Corrects 00001's photo_paths cap: 10*512+9=5129, not 5120.
ALTER TABLE sightings
    DROP CONSTRAINT sightings_photo_paths_check;
ALTER TABLE sightings
    ADD CONSTRAINT sightings_photo_paths_check
        CHECK (cardinality(photo_paths) <= 10
           AND octet_length(array_to_string(photo_paths, '/')) <= 5129);

-- Helper function for Supabase storage.objects SELECT policies.
-- SECURITY DEFINER bypasses table RLS during group membership checks.
-- Avoids auth.* / storage.* dependencies for standard Postgres CI compatibility.
CREATE FUNCTION can_view_sighting_media(viewer_auth_id uuid, object_name text)
RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM sightings s
        JOIN users owner ON owner.id = s.user_id
        JOIN group_members gm_owner ON gm_owner.user_id = owner.id
        JOIN group_members gm_viewer ON gm_viewer.group_id = gm_owner.group_id
        JOIN users viewer ON viewer.id = gm_viewer.user_id
        WHERE viewer.auth_id = viewer_auth_id
          AND owner.auth_id::text = split_part(object_name, '/', 1)
          AND s.id = split_part(object_name, '/', 2)
          AND s.deleted_at IS NULL
          AND (object_name = ANY(s.photo_paths) OR object_name = ANY(s.recording_paths))
    );
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS can_view_sighting_media(uuid, text);
ALTER TABLE sightings
    DROP CONSTRAINT sightings_photo_paths_check;
ALTER TABLE sightings
    ADD CONSTRAINT sightings_photo_paths_check
        CHECK (cardinality(photo_paths) <= 10
           AND octet_length(array_to_string(photo_paths, '/')) <= 5120);
ALTER TABLE sightings DROP COLUMN IF EXISTS recording_paths;
-- +goose StatementEnd
