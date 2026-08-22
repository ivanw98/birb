-- +goose Up
-- +goose StatementBegin

-- Seeded from GeoNames by 00006; see tools/README.md. geonames_id is the natural
-- key a dataset refresh matches on, so UNIQUE here fails a renumbering bug loudly.
CREATE TABLE places (
    id           text PRIMARY KEY CHECK (id ~ '^plc_[a-z0-9]{26}$'),
    geonames_id  integer NOT NULL UNIQUE,
    name         text NOT NULL CHECK (char_length(name) <= 200),
    latitude     double precision NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude    double precision NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    population   integer NOT NULL CHECK (population >= 0),
    feature_code text NOT NULL
);

CREATE INDEX places_lat_lon_idx ON places (latitude, longitude);

ALTER TABLE places ENABLE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS places;
-- +goose StatementEnd
