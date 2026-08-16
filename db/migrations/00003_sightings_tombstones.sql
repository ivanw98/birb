-- +goose Up
-- +goose StatementBegin

-- Serves GET /api/sightings?includeDeleted=true
CREATE INDEX sightings_user_pull_idx
    ON sightings (user_id, observed_at DESC, id DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS sightings_user_pull_idx;
-- +goose StatementEnd
