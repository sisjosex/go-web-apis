-- TRACK-007 step 1 (D1): a stop is a place, not a row on one route, and a route's list of stops is
-- versioned so a past trip keeps the list it actually ran.
--
-- `route_stops` is replaced, not kept beside the new tables. Every row copies into `stop_places`
-- **with the same id**, so `rider_assignments.pickup_stop_id`, `rider_assignments.dropoff_stop_id`
-- and `ride_events.stop_id` are only re-pointed — no data moves, nothing is rewritten, and there is
-- one source of truth for what a stop is.
--
-- Each route then gets version 1 in its old `stop_order`, effective from the day the route was
-- created: that is as far back as we can honestly claim the list was in force.
--
-- PostGIS carries the coordinates (the image is `postgis/postgis:17-3.5` locally and in prod), so
-- "the stops within 500 m of here" is an index scan rather than a table-wide haversine. btree_gist
-- is what lets `route_versions` exclude two overlapping ranges *for the same route* — the equality
-- half of that constraint needs a GIST operator class for UUID.

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ===========================================================================
-- Tables
-- ===========================================================================

-- A corner two routes both call at is one row here, named once. organization_id is the school or
-- employer whose gate this is, and NULL for a public stop that belongs to no one.
CREATE TABLE tracking.stop_places (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    organization_id UUID REFERENCES tracking.organizations(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    address VARCHAR(500),
    -- Nullable only because the rows this migration backfills may carry no coordinates: a legacy
    -- stop keeps its name and is fixed by an edit rather than being dropped. Every stop place
    -- created through the API has one — the DTO requires it.
    location GEOGRAPHY(Point, 4326),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_stop_places_location ON tracking.stop_places USING GIST (location);
-- Filter + order in one index, so a page is not a sort of the whole table; the trigram index serves
-- the '%term%' search, which no b-tree can.
CREATE INDEX idx_stop_places_tenant_name ON tracking.stop_places (tenant_id, name, id);
CREATE INDEX idx_stop_places_name_trgm ON tracking.stop_places USING GIN (name gin_trgm_ops);
CREATE INDEX idx_stop_places_organization_id ON tracking.stop_places (organization_id)
    WHERE organization_id IS NOT NULL;

-- One route's list of stops, in force over a closed or open-ended range of dates. The exclusion
-- constraint is the whole invariant: a route can never have two lists in force on the same day, so
-- "which stops ran on the 4th" has exactly one answer without the reader taking a lock.
CREATE TABLE tracking.route_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    effective_from DATE NOT NULL,
    effective_to DATE,
    created_by UUID,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_route_version_range CHECK (effective_to IS NULL OR effective_to >= effective_from),
    CONSTRAINT ex_route_version_overlap EXCLUDE USING GIST (
        route_id WITH =,
        daterange(effective_from, effective_to, '[]') WITH &&
    )
);

CREATE INDEX idx_route_versions_route_from ON tracking.route_versions (route_id, effective_from DESC);

-- The list itself. UNIQUE (version_id, sequence) is what makes a whole-list reorder one statement:
-- the editor replaces the list, it never renumbers rows one at a time.
CREATE TABLE tracking.route_version_stops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id UUID NOT NULL REFERENCES tracking.route_versions(id) ON DELETE CASCADE,
    stop_place_id UUID NOT NULL REFERENCES tracking.stop_places(id),
    sequence INT NOT NULL,
    -- Minutes after the route's scheduled start that the bus is due here, and how long it waits.
    planned_offset_min INT,
    dwell_sec INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_route_version_stop_sequence UNIQUE (version_id, sequence),
    CONSTRAINT chk_route_version_stop_sequence CHECK (sequence >= 1)
);

CREATE INDEX idx_route_version_stops_stop_place ON tracking.route_version_stops (stop_place_id);

COMMENT ON TABLE tracking.stop_places IS
'A place a route calls at, named once and shared by every route that stops there (TRACK-007 D1); location is a PostGIS point so "near here" is an index scan';

COMMENT ON TABLE tracking.route_versions IS
'One route''s stop list over a range of dates; the exclusion constraint guarantees at most one version is in force per route per day (TRACK-007)';

COMMENT ON TABLE tracking.route_version_stops IS
'The ordered stops of one route version; a reorder replaces the whole list in one request rather than renumbering rows';

-- ===========================================================================
-- Routes gain a direction (TRACK-007). `default_driver_id` the spec also names is already here,
-- added by TRACK-006 (20260921000001), so it is not re-added.
-- ===========================================================================
ALTER TABLE tracking.routes
    ADD COLUMN direction VARCHAR(20) NOT NULL DEFAULT 'outbound',
    ADD CONSTRAINT chk_routes_direction CHECK (direction IN ('outbound', 'inbound'));

COMMENT ON COLUMN tracking.routes.direction IS
'Which way this route runs: outbound leaves the depot, inbound returns (TRACK-007)';

-- ===========================================================================
-- Backfill (D1): same-id copy, then one version per route in the old order
-- ===========================================================================

-- The same corner duplicated across routes stays duplicated: merging two rows would silently
-- rewrite which stop an assignment points at. The operator merges them by hand afterwards.
INSERT INTO tracking.stop_places (id, tenant_id, organization_id, name, address, location, created_at, updated_at)
SELECT
    rs.id,
    tc.tenant_id,
    NULL,
    COALESCE(NULLIF(TRIM(rs.location_name), ''), 'Stop ' || rs.stop_order),
    NULLIF(TRIM(rs.location_name), ''),
    CASE
        WHEN rs.latitude IS NOT NULL AND rs.longitude IS NOT NULL
        THEN ST_SetSRID(ST_MakePoint(rs.longitude::float8, rs.latitude::float8), 4326)::geography
    END,
    rs.created_at,
    rs.updated_at
FROM tracking.route_stops rs
INNER JOIN tracking.routes r ON r.id = rs.route_id
INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id;

-- Every route gets version 1, including one with no stops: the editor then has a list to fill
-- rather than a route with nowhere to put one.
INSERT INTO tracking.route_versions (route_id, effective_from, effective_to, created_by)
SELECT r.id, r.created_at::date, NULL, NULL
FROM tracking.routes r;

-- ROW_NUMBER rather than stop_order itself: route_stops never enforced that the order was unique
-- within a route, and uk_route_version_stop_sequence does.
INSERT INTO tracking.route_version_stops (version_id, stop_place_id, sequence, created_at, updated_at)
SELECT
    v.id,
    rs.id,
    CAST(ROW_NUMBER() OVER (PARTITION BY v.id ORDER BY rs.stop_order, rs.id) AS INT),
    rs.created_at,
    rs.updated_at
FROM tracking.route_versions v
INNER JOIN tracking.route_stops rs ON rs.route_id = v.route_id;

-- ===========================================================================
-- Re-point what referenced a route stop. The ids did not change, so this is constraints only.
-- rider_assignments carries two foreign keys per column — an inline REFERENCES and a named
-- CONSTRAINT, both from 20251203000007 — so both names are dropped.
-- ===========================================================================
ALTER TABLE tracking.rider_assignments
    DROP CONSTRAINT IF EXISTS fk_assignment_pickup,
    DROP CONSTRAINT IF EXISTS fk_assignment_dropoff,
    DROP CONSTRAINT IF EXISTS rider_assignments_pickup_stop_id_fkey,
    DROP CONSTRAINT IF EXISTS rider_assignments_dropoff_stop_id_fkey,
    ADD CONSTRAINT fk_assignment_pickup FOREIGN KEY (pickup_stop_id)
        REFERENCES tracking.stop_places(id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_assignment_dropoff FOREIGN KEY (dropoff_stop_id)
        REFERENCES tracking.stop_places(id) ON DELETE SET NULL;

ALTER TABLE tracking.ride_events
    DROP CONSTRAINT IF EXISTS ride_events_stop_id_fkey,
    ADD CONSTRAINT fk_ride_event_stop FOREIGN KEY (stop_id)
        REFERENCES tracking.stop_places(id) ON DELETE SET NULL;

-- ===========================================================================
-- The SPs that read a route stop. sp_create_route_stop and sp_delete_route_stop go with the table:
-- a stop is created as a place and put on a route by a version now, never in one call.
-- ===========================================================================
DROP FUNCTION IF EXISTS tracking.sp_create_route_stop(UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, INT, TIME);
DROP FUNCTION IF EXISTS tracking.sp_delete_route_stop(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_route_stops(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_rider_status(UUID, UUID, UUID, UUID);

DROP TABLE tracking.route_stops;

-- A route out of scope must refuse rather than return an empty list: an empty stop list reads as
-- "this route has no stops", which is a different and misleading fact.
--
-- p_date is the day whose list is wanted, today when unset (D1). The version in force that day is
-- the only one that matches, so the extra join costs one index lookup and the answer is a fact
-- about a date rather than about "now".
CREATE FUNCTION tracking.sp_list_route_stops(
    p_tenant_id UUID,
    p_route_id UUID,
    p_scope_user_id UUID DEFAULT NULL,
    p_date DATE DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    version_id UUID,
    stop_place_id UUID,
    stop_name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    sequence INT,
    planned_offset_min INT,
    dwell_sec INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_scope UUID[];
    v_date DATE := COALESCE(p_date, CURRENT_DATE);
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);

        IF NOT EXISTS (
            SELECT 1 FROM tracking.routes r
            INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
            WHERE r.id = p_route_id
              AND tc.tenant_id = p_tenant_id
              AND EXISTS (
                  SELECT 1
                  FROM tracking.rider_assignments ra
                  INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
                  WHERE ra.route_id = r.id
                    AND rr.organization_id = ANY(v_scope)
              )
        ) THEN
            RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    SELECT
        vs.id,
        rv.route_id,
        rv.id,
        sp.id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        vs.sequence,
        vs.planned_offset_min,
        vs.dwell_sec,
        vs.created_at,
        vs.updated_at
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    INNER JOIN tracking.route_version_stops vs ON vs.version_id = rv.id
    INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id
    WHERE rv.route_id = p_route_id
      AND tc.tenant_id = p_tenant_id
      AND rv.effective_from <= v_date
      AND (rv.effective_to IS NULL OR rv.effective_to >= v_date)
    ORDER BY vs.sequence ASC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_route_stops(UUID, UUID, UUID, DATE) IS
'The stops a route runs on p_date (today when unset), read from the version in force that day (TRACK-007 D1); a route outside p_scope_user_id''s organizations raises route.not-found';

-- sp_get_rider_status reads the three stop names it shows from stop_places now. Nothing else about
-- it changes: the ids it joins on are the same ids.
CREATE FUNCTION tracking.sp_get_rider_status(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_scope_user_id UUID DEFAULT NULL,
    p_guardian_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_name VARCHAR,
    vehicle_latitude DECIMAL,
    vehicle_longitude DECIMAL,
    vehicle_speed DECIMAL,
    location_age_seconds INT,
    last_event_type VARCHAR,
    last_event_time TIMESTAMP,
    last_event_notes TEXT,
    last_event_stop VARCHAR,
    scheduled_pickup_stop VARCHAR,
    scheduled_dropoff_stop VARCHAR,
    active_alerts INT
) AS $$
DECLARE
    v_scope UUID[];
    v_guardian_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
          AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope))
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rider.id AS rider_id,
        CAST(rider.first_name || ' ' || rider.last_name AS VARCHAR) AS rider_name,
        route.id AS route_id,
        CAST(route.route_name AS VARCHAR),
        v.id AS vehicle_id,
        CAST(v.plate_number AS VARCHAR) AS license_plate,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR) AS driver_name,
        vl.latitude AS vehicle_latitude,
        vl.longitude AS vehicle_longitude,
        vl.speed AS vehicle_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        CAST(last_event.event_type AS VARCHAR) AS last_event_type,
        last_event.event_time AS last_event_time,
        last_event.notes AS last_event_notes,
        CAST(last_event.stop_name AS VARCHAR) AS last_event_stop,
        CAST(pickup_stop.name AS VARCHAR) AS scheduled_pickup_stop,
        CAST(dropoff_stop.name AS VARCHAR) AS scheduled_dropoff_stop,
        COALESCE((
            SELECT COUNT(*)::INT
            FROM tracking.route_alerts alerts
            WHERE alerts.route_id = ra.route_id
              AND alerts.status = 'active'
        ), 0) AS active_alerts
    FROM tracking.riders rider
    LEFT JOIN LATERAL (
        SELECT ra_inner.route_id, ra_inner.pickup_stop_id, ra_inner.dropoff_stop_id
        FROM tracking.rider_assignments ra_inner
        INNER JOIN tracking.routes ro ON ro.id = ra_inner.route_id
        WHERE ra_inner.rider_id = rider.id
          AND ra_inner.status = 'active'
          AND ro.scheduled_end_time >= LOCALTIME
        ORDER BY ro.scheduled_start_time, ro.id
        LIMIT 1
    ) ra ON true
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.vehicles v ON v.id = route.vehicle_id
    LEFT JOIN tracking.drivers d ON d.id = route.default_driver_id
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    LEFT JOIN LATERAL (
        SELECT event_inner.event_type, event_inner.event_time, event_inner.notes, event_stop.name AS stop_name
        FROM tracking.ride_events event_inner
        LEFT JOIN tracking.stop_places event_stop ON event_stop.id = event_inner.stop_id
        WHERE event_inner.rider_id = p_rider_id
        ORDER BY event_inner.event_time DESC, event_inner.created_at DESC
        LIMIT 1
    ) last_event ON true
    LEFT JOIN tracking.stop_places pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.stop_places dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_get_rider_status(UUID, UUID, UUID, UUID) IS
'A rider''s live status for the guardian view, scoped to p_scope_user_id''s organizations and to p_guardian_user_id''s own riders when either is set; out of scope raises rider.not-found. The three stop names come from stop_places (TRACK-007 D1) and driver_name is the route''s default driver (TRACK-006)';
