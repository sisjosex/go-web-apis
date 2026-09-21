-- Reverse of TRACK-007 step 1: `route_stops` comes back carrying the same ids it handed over, the
-- foreign keys point at it again, and the three new tables go.
--
-- The one thing this cannot undo is the sharing itself: a stop place that two routes both call at
-- was one row and has to become one `route_stops` row again, on whichever route reaches it first.
-- The same is true of history — only the version in force today is restored, because `route_stops`
-- has nowhere to put the others. A reference to a stop place that no restored row covers is set to
-- NULL rather than blocking the rollback.
--
-- The SPs this migration replaced are restored verbatim from 20260407000004, 20260920000001 and
-- 20260923000003: a dropped function is not recreated by re-running an older migration, so the down
-- migration carries them itself.

CREATE TABLE tracking.route_stops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    stop_order INT NOT NULL,
    location_name VARCHAR(255),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    estimated_arrival TIME,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_stop_route FOREIGN KEY (route_id) REFERENCES tracking.routes(id)
);

CREATE INDEX idx_route_stops_route_id ON tracking.route_stops(route_id);

INSERT INTO tracking.route_stops (id, route_id, stop_order, location_name, latitude, longitude, status, created_at, updated_at)
SELECT DISTINCT ON (sp.id)
    sp.id,
    rv.route_id,
    vs.sequence,
    sp.name,
    CAST(ST_Y(sp.location::geometry) AS DECIMAL(10, 8)),
    CAST(ST_X(sp.location::geometry) AS DECIMAL(11, 8)),
    'active',
    sp.created_at,
    sp.updated_at
FROM tracking.stop_places sp
INNER JOIN tracking.route_version_stops vs ON vs.stop_place_id = sp.id
INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
WHERE rv.effective_from <= CURRENT_DATE
  AND (rv.effective_to IS NULL OR rv.effective_to >= CURRENT_DATE)
ORDER BY sp.id, rv.route_id, vs.sequence;

UPDATE tracking.rider_assignments ra
   SET pickup_stop_id = NULL
 WHERE ra.pickup_stop_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM tracking.route_stops rs WHERE rs.id = ra.pickup_stop_id);

UPDATE tracking.rider_assignments ra
   SET dropoff_stop_id = NULL
 WHERE ra.dropoff_stop_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM tracking.route_stops rs WHERE rs.id = ra.dropoff_stop_id);

UPDATE tracking.ride_events re
   SET stop_id = NULL
 WHERE re.stop_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM tracking.route_stops rs WHERE rs.id = re.stop_id);

ALTER TABLE tracking.rider_assignments
    DROP CONSTRAINT IF EXISTS fk_assignment_pickup,
    DROP CONSTRAINT IF EXISTS fk_assignment_dropoff,
    ADD CONSTRAINT fk_assignment_pickup FOREIGN KEY (pickup_stop_id)
        REFERENCES tracking.route_stops(id),
    ADD CONSTRAINT fk_assignment_dropoff FOREIGN KEY (dropoff_stop_id)
        REFERENCES tracking.route_stops(id);

ALTER TABLE tracking.ride_events
    DROP CONSTRAINT IF EXISTS fk_ride_event_stop,
    ADD CONSTRAINT ride_events_stop_id_fkey FOREIGN KEY (stop_id)
        REFERENCES tracking.route_stops(id) ON DELETE SET NULL;

DROP TABLE IF EXISTS tracking.route_version_stops;
DROP TABLE IF EXISTS tracking.route_versions;
DROP TABLE IF EXISTS tracking.stop_places;

ALTER TABLE tracking.routes
    DROP CONSTRAINT IF EXISTS chk_routes_direction,
    DROP COLUMN IF EXISTS direction;

DROP FUNCTION IF EXISTS tracking.sp_list_route_stops(UUID, UUID, UUID, DATE);
DROP FUNCTION IF EXISTS tracking.sp_get_rider_status(UUID, UUID, UUID, UUID);

-- ===========================================================================
-- sp_create_route_stop / sp_delete_route_stop — 20260407000004
-- ===========================================================================
CREATE FUNCTION tracking.sp_create_route_stop(
    p_tenant_id UUID,
    p_route_id UUID,
    p_stop_name VARCHAR(255),
    p_stop_address VARCHAR(500),
    p_latitude DECIMAL(10,8),
    p_longitude DECIMAL(11,8),
    p_sequence_order INT,
    p_scheduled_time TIME
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    stop_name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    stop_order INT,
    scheduled_arrival_offset_minutes INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.route_stops (
        route_id, location_name, stop_order, latitude, longitude
    )
    VALUES (
        p_route_id,
        TRIM(p_stop_name),
        COALESCE(p_sequence_order, 0),
        p_latitude,
        p_longitude
    )
    RETURNING
        tracking.route_stops.id,
        tracking.route_stops.route_id,
        CAST(tracking.route_stops.location_name AS VARCHAR),
        CAST(tracking.route_stops.location_name AS VARCHAR),
        tracking.route_stops.latitude,
        tracking.route_stops.longitude,
        tracking.route_stops.stop_order,
        NULL::INT,
        tracking.route_stops.created_at,
        tracking.route_stops.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_delete_route_stop(
    p_tenant_id UUID,
    p_stop_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.route_stops rs
        INNER JOIN tracking.routes r ON r.id = rs.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE rs.id = p_stop_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route-stop.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.route_stops rs WHERE rs.id = p_stop_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_list_route_stops — 20260920000001
-- ===========================================================================
CREATE FUNCTION tracking.sp_list_route_stops(
    p_tenant_id UUID,
    p_route_id UUID,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    stop_name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    stop_order INT,
    scheduled_arrival_offset_minutes INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_scope UUID[];
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
        rs.id,
        rs.route_id,
        CAST(rs.location_name AS VARCHAR),
        CAST(rs.location_name AS VARCHAR),
        rs.latitude,
        rs.longitude,
        rs.stop_order,
        NULL::INT,
        rs.created_at,
        rs.updated_at
    FROM tracking.route_stops rs
    INNER JOIN tracking.routes r ON r.id = rs.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rs.route_id = p_route_id AND tc.tenant_id = p_tenant_id
    ORDER BY rs.stop_order ASC;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_get_rider_status — 20260923000003
-- ===========================================================================
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
        CAST(pickup_stop.location_name AS VARCHAR) AS scheduled_pickup_stop,
        CAST(dropoff_stop.location_name AS VARCHAR) AS scheduled_dropoff_stop,
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
        SELECT event_inner.event_type, event_inner.event_time, event_inner.notes, event_stop.location_name AS stop_name
        FROM tracking.ride_events event_inner
        LEFT JOIN tracking.route_stops event_stop ON event_stop.id = event_inner.stop_id
        WHERE event_inner.rider_id = p_rider_id
        ORDER BY event_inner.event_time DESC, event_inner.created_at DESC
        LIMIT 1
    ) last_event ON true
    LEFT JOIN tracking.route_stops pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.route_stops dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_get_rider_status(UUID, UUID, UUID, UUID) IS
'A rider''s live status for the guardian view, scoped to p_scope_user_id''s organizations and to p_guardian_user_id''s own riders when either is set; out of scope raises rider.not-found. driver_name is the route''s default driver (TRACK-006) and is NULL when the route has none';
