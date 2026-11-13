BEGIN;

CREATE TABLE geofences (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL CHECK (category IN ('delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area')),
    coordinates JSONB NOT NULL,
    geometry geometry(Polygon, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX geofences_geometry_idx ON geofences USING GiST (geometry);
CREATE INDEX geofences_category_idx ON geofences (category, id);

CREATE TABLE vehicles (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vehicle_number TEXT NOT NULL,
    driver_name TEXT NOT NULL,
    vehicle_type TEXT NOT NULL,
    phone TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_latitude DOUBLE PRECISION CHECK (last_latitude BETWEEN -90 AND 90),
    last_longitude DOUBLE PRECISION CHECK (last_longitude BETWEEN -180 AND 180),
    last_location_at TIMESTAMPTZ,
    CHECK (num_nonnulls(last_latitude, last_longitude, last_location_at) IN (0, 3))
);
-- Trim the same Unicode whitespace characters as Go strings.TrimSpace.
CREATE UNIQUE INDEX vehicles_number_unique_idx
    ON vehicles (lower(btrim(vehicle_number, U&'\0009\000A\000B\000C\000D\0020\0085\00A0\1680\2000\2001\2002\2003\2004\2005\2006\2007\2008\2009\200A\2028\2029\202F\205F\3000')));

CREATE TABLE vehicle_locations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vehicle_id BIGINT NOT NULL REFERENCES vehicles(id),
    latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    recorded_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX vehicle_locations_vehicle_time_idx ON vehicle_locations (vehicle_id, recorded_at DESC);

CREATE TABLE vehicle_geofence_memberships (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vehicle_id BIGINT NOT NULL REFERENCES vehicles(id),
    geofence_id BIGINT NOT NULL REFERENCES geofences(id),
    UNIQUE (vehicle_id, geofence_id)
);

CREATE TABLE violations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vehicle_id BIGINT NOT NULL REFERENCES vehicles(id),
    geofence_id BIGINT NOT NULL REFERENCES geofences(id),
    event_type TEXT NOT NULL CHECK (event_type IN ('entry', 'exit')),
    latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    timestamp TIMESTAMPTZ NOT NULL
);
CREATE INDEX violations_time_idx ON violations (timestamp DESC, id DESC);
CREATE INDEX violations_vehicle_time_idx ON violations (vehicle_id, timestamp DESC, id DESC);
CREATE INDEX violations_geofence_time_idx ON violations (geofence_id, timestamp DESC, id DESC);

CREATE TABLE alert_rules (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    geofence_id BIGINT NOT NULL REFERENCES geofences(id),
    vehicle_id BIGINT REFERENCES vehicles(id),
    event_type TEXT NOT NULL CHECK (event_type IN ('entry', 'exit', 'both')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX alert_rules_active_geofence_idx ON alert_rules (geofence_id) WHERE status = 'active';

CREATE TABLE alerts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    violation_id BIGINT NOT NULL UNIQUE REFERENCES violations(id),
    matched_rule_ids BIGINT[] NOT NULL CHECK (cardinality(matched_rule_ids) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
