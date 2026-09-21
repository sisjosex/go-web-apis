-- Reverses TRACK-008 step 1: ride_events comes back with every row it held, rebuilt from the whole
-- copy each one left in trip_events (payload.ride_event) — ids, coordinates and notes included.
-- Transitions recorded on trips after the migration have no such copy and do not come back.

CREATE TABLE tracking.ride_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rider_id UUID NOT NULL,
    vehicle_id UUID NOT NULL,
    route_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    event_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    location_latitude DECIMAL(10, 8),
    location_longitude DECIMAL(11, 8),
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    stop_id UUID,
    CONSTRAINT chk_event_type CHECK (event_type IN ('check_in', 'checkout', 'no_show', 'emergency')),
    CONSTRAINT fk_event_rider FOREIGN KEY (rider_id) REFERENCES tracking.riders(id) ON DELETE CASCADE,
    CONSTRAINT fk_event_vehicle FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id) ON DELETE CASCADE,
    CONSTRAINT fk_event_route FOREIGN KEY (route_id) REFERENCES tracking.routes(id) ON DELETE CASCADE,
    CONSTRAINT fk_ride_event_stop FOREIGN KEY (stop_id) REFERENCES tracking.stop_places(id) ON DELETE SET NULL
);

CREATE INDEX idx_ride_events_rider_id ON tracking.ride_events(rider_id);
CREATE INDEX idx_ride_events_vehicle_id ON tracking.ride_events(vehicle_id);
CREATE INDEX idx_ride_events_route_id ON tracking.ride_events(route_id);
CREATE INDEX idx_ride_events_stop_id ON tracking.ride_events(stop_id) WHERE stop_id IS NOT NULL;

INSERT INTO tracking.ride_events
SELECT (jsonb_populate_record(NULL::tracking.ride_events, te.payload->'ride_event')).*
FROM tracking.trip_events te
WHERE te.payload ? 'ride_event';

DROP TABLE tracking.trip_events;
DROP TABLE tracking.trip_stop_tasks;
DROP TABLE tracking.trip_stops;
DROP TABLE tracking.trips;

-- sp_record_ride_event — 20260918000003

CREATE FUNCTION tracking.sp_record_ride_event(
    p_rider_id UUID,
    p_vehicle_id UUID,
    p_route_id UUID,
    p_event_type VARCHAR(50),
    p_location_latitude DECIMAL(10, 8),
    p_location_longitude DECIMAL(11, 8),
    p_notes TEXT,
    p_created_by UUID,
    p_stop_id UUID
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    vehicle_id UUID,
    route_id UUID,
    event_type VARCHAR(50),
    event_time TIMESTAMP,
    location_latitude DECIMAL(10, 8),
    location_longitude DECIMAL(11, 8),
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP
) AS $$
BEGIN
    IF p_event_type NOT IN ('check_in', 'checkout', 'no_show', 'emergency') THEN
        RAISE EXCEPTION 'event.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM tracking.riders tr WHERE tr.id = p_rider_id) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles tv WHERE tv.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM tracking.routes tr WHERE tr.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.ride_events (
        rider_id, vehicle_id, route_id, stop_id, event_type, event_time, location_latitude, location_longitude, notes, created_by, created_at
    )
    VALUES (
        p_rider_id, p_vehicle_id, p_route_id, p_stop_id, p_event_type, CURRENT_TIMESTAMP, p_location_latitude, p_location_longitude, p_notes, p_created_by, CURRENT_TIMESTAMP
    )
    RETURNING
        tracking.ride_events.id,
        tracking.ride_events.rider_id,
        tracking.ride_events.vehicle_id,
        tracking.ride_events.route_id,
        tracking.ride_events.event_type,
        tracking.ride_events.event_time,
        tracking.ride_events.location_latitude,
        tracking.ride_events.location_longitude,
        tracking.ride_events.notes,
        tracking.ride_events.created_by,
        tracking.ride_events.created_at;
END;
$$ LANGUAGE plpgsql;

-- sp_update_route and sp_get_route without the zone — 20260407000003, 20260920000001

DROP FUNCTION IF EXISTS tracking.sp_update_route(UUID, UUID, VARCHAR, UUID, BOOLEAN, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_get_route(UUID, UUID, UUID);

CREATE FUNCTION tracking.sp_update_route(
    p_tenant_id UUID,
    p_route_id UUID,
    p_route_name VARCHAR(255),
    p_vehicle_id UUID,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR,
    route_code VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    schedule_type VARCHAR,
    scheduled_start_time TIME,
    scheduled_end_time TIME,
    estimated_duration_minutes INT,
    is_active BOOLEAN,
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
    UPDATE tracking.routes r SET
        route_name = COALESCE(NULLIF(TRIM(p_route_name), ''), r.route_name),
        vehicle_id = p_vehicle_id,
        is_active = COALESCE(p_is_active, r.is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_route_id
    RETURNING
        r.id,
        r.company_id,
        r.vehicle_id,
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        CAST(r.schedule_type AS VARCHAR),
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_get_route(
    p_tenant_id UUID,
    p_route_id UUID,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR,
    route_code VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    schedule_type VARCHAR,
    scheduled_start_time TIME,
    scheduled_end_time TIME,
    estimated_duration_minutes INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id
          AND tc.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR EXISTS (
              SELECT 1
              FROM tracking.rider_assignments ra
              INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
              WHERE ra.route_id = r.id
                AND rr.organization_id = ANY(v_scope)
          ))
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        r.vehicle_id,
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        CAST(r.schedule_type AS VARCHAR),
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at
    FROM tracking.routes r
    WHERE r.id = p_route_id;
END;
$$ LANGUAGE plpgsql;

ALTER TABLE tracking.routes DROP COLUMN timezone;

COMMENT ON TABLE tracking.ride_events IS
'Boarding, arrival, no-show and emergency events per rider and route (restored by the TRACK-008 down migration)';
