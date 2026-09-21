-- TRACK-008 step 1: the trip — "route X on 2026-10-03" — becomes a row, and ride_events becomes its
-- history.
--
-- A trip is one departure of one schedule on one service day (TRACK-018: the key is
-- route_schedule_id + service_date). Its stops are copied from the route version in force that day,
-- and a stop holds tasks, not riders (ARCH-001 D1): pickup and dropoff, of a passenger — or, one day,
-- of a parcel. `parcel` is in the CHECK so that day needs no migration; every SP refuses it until a
-- delivery module exists.
--
-- D2: `service_date` is the route's local day. A route runs in one IANA zone, and the trip copies it
-- so that editing the route's zone later never moves a trip already built.
--
-- D3: ride_events is replaced, not kept beside the trips. Each (route, local day) with events becomes
-- one completed trip; check_in is a pickup done, checkout a dropoff done, no_show a pickup no_show.
-- The stop is the event's own, else the rider's assigned one, else the first (pickup) or last
-- (dropoff) — whichever of those is on the trip's list first. Every legacy row is also copied whole
-- into trip_events: that is where emergency lands, and it is what the down migration rebuilds from,
-- because a task cannot hold two events for the same rider or an event's coordinates.

-- ===========================================================================
-- Route timezone (D2)
-- ===========================================================================

ALTER TABLE tracking.routes
    ADD COLUMN timezone VARCHAR(64) NOT NULL DEFAULT 'UTC';

COMMENT ON COLUMN tracking.routes.timezone IS
'IANA zone the route runs in: its trips'' service_date is the local day and planned_start is service_date + start time in this zone (TRACK-008 D2)';

-- ===========================================================================
-- Tables
-- ===========================================================================

CREATE TABLE tracking.trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    route_version_id UUID REFERENCES tracking.route_versions(id) ON DELETE SET NULL,
    -- NULL for a trip rebuilt from ride_events, which no schedule produced, and for a trip whose
    -- schedule was deleted after it ran.
    route_schedule_id UUID REFERENCES tracking.route_schedules(id) ON DELETE SET NULL,
    service_date DATE NOT NULL,
    timezone VARCHAR(64) NOT NULL,
    planned_start TIMESTAMPTZ NOT NULL,
    vehicle_id UUID REFERENCES tracking.vehicles(id) ON DELETE SET NULL,
    driver_id UUID REFERENCES tracking.drivers(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'planned',
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    -- The operator owns the header: the materialiser no longer rewrites vehicle, driver, start or
    -- status. Stops and tasks still follow the plan until the trip starts (D4).
    is_overridden BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_trips_status CHECK (status IN ('planned', 'in_progress', 'completed', 'cancelled')),
    CONSTRAINT uk_trips_schedule_date UNIQUE (route_schedule_id, service_date)
);

CREATE INDEX idx_trips_route_date ON tracking.trips (route_id, service_date);
CREATE INDEX idx_trips_tenant_date_status ON tracking.trips (tenant_id, service_date, status);

CREATE TABLE tracking.trip_stops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES tracking.trips(id) ON DELETE CASCADE,
    stop_place_id UUID NOT NULL REFERENCES tracking.stop_places(id),
    sequence INT NOT NULL,
    planned_at TIMESTAMPTZ,
    arrived_at TIMESTAMPTZ,
    departed_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_trip_stops_status CHECK (status IN ('pending', 'arrived', 'skipped')),
    -- Also the trip_id index: every read of a trip's stops is by this prefix.
    CONSTRAINT uk_trip_stops_sequence UNIQUE (trip_id, sequence)
);

CREATE INDEX idx_trip_stops_stop_place ON tracking.trip_stops (stop_place_id);

CREATE TABLE tracking.trip_stop_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_stop_id UUID NOT NULL REFERENCES tracking.trip_stops(id) ON DELETE CASCADE,
    kind VARCHAR(20) NOT NULL,
    subject_type VARCHAR(20) NOT NULL DEFAULT 'passenger',
    -- A rider for a passenger. Not a foreign key: the column is polymorphic by subject_type.
    subject_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    done_at TIMESTAMPTZ,
    done_by UUID,
    client_op_id UUID UNIQUE,
    proof JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_trip_stop_tasks_kind CHECK (kind IN ('pickup', 'dropoff')),
    CONSTRAINT chk_trip_stop_tasks_subject CHECK (subject_type IN ('passenger', 'parcel')),
    CONSTRAINT chk_trip_stop_tasks_status CHECK (status IN ('pending', 'done', 'no_show', 'cancelled')),
    -- Also the trip_stop_id index; it is the materialiser's diff key.
    CONSTRAINT uk_trip_stop_tasks_subject UNIQUE (trip_stop_id, kind, subject_type, subject_id)
);

-- A rider's last transition, whatever trip it was on: one index probe for the guardian view.
CREATE INDEX idx_trip_stop_tasks_subject_done ON tracking.trip_stop_tasks (subject_id, done_at DESC)
    WHERE done_at IS NOT NULL;

CREATE TABLE tracking.trip_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES tracking.trips(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID
);

CREATE INDEX idx_trip_events_trip ON tracking.trip_events (trip_id, created_at);

COMMENT ON TABLE tracking.trips IS
'One departure of a route on one service day, keyed by schedule and local date (TRACK-008); built by sp_materialise_trips from the TRACK-018 plan';
COMMENT ON TABLE tracking.trip_stops IS
'The stops of one trip, copied from the route version in force on its service date';
COMMENT ON TABLE tracking.trip_stop_tasks IS
'What happens at a stop: a pickup or dropoff of one subject (ARCH-001 D1: parcel is reserved, refused by every SP until a delivery module exists)';
COMMENT ON TABLE tracking.trip_events IS
'A trip''s audit trail; ride_events rows migrated by TRACK-008 carry the whole legacy row under payload.ride_event';

-- ===========================================================================
-- Backfill from ride_events (D3)
-- ===========================================================================

-- event_time was written as CURRENT_TIMESTAMP into a column without a zone, so it is a wall-clock
-- time in the server's zone: read it back in that zone before asking for the route's local day.
CREATE TEMP TABLE legacy_events AS
SELECT
    re.*,
    re.event_time AT TIME ZONE current_setting('TimeZone') AS event_at,
    CAST((re.event_time AT TIME ZONE current_setting('TimeZone')) AT TIME ZONE r.timezone AS DATE) AS service_date,
    r.timezone,
    tc.tenant_id
FROM tracking.ride_events re
INNER JOIN tracking.routes r ON r.id = re.route_id
INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id;

INSERT INTO tracking.trips (
    tenant_id, route_id, route_version_id, route_schedule_id, service_date, timezone, planned_start,
    vehicle_id, driver_id, status, started_at, ended_at
)
SELECT
    le.tenant_id,
    le.route_id,
    (SELECT rv.id FROM tracking.route_versions rv
      WHERE rv.route_id = le.route_id
        AND le.service_date >= rv.effective_from
        AND (rv.effective_to IS NULL OR le.service_date <= rv.effective_to)),
    NULL,
    le.service_date,
    le.timezone,
    MIN(le.event_at),
    (array_agg(le.vehicle_id ORDER BY le.event_at DESC))[1],
    NULL,
    'completed',
    MIN(le.event_at),
    MAX(le.event_at)
FROM legacy_events le
GROUP BY le.tenant_id, le.route_id, le.service_date, le.timezone;

INSERT INTO tracking.trip_stops (trip_id, stop_place_id, sequence, planned_at, status)
SELECT t.id, vs.stop_place_id, vs.sequence, NULL, 'pending'
FROM tracking.trips t
INNER JOIN tracking.route_version_stops vs ON vs.version_id = t.route_version_id
WHERE t.route_schedule_id IS NULL AND t.status = 'completed';

-- One task per (stop, kind, rider); when a rider has several events for the same task that day the
-- latest one is the state, and the rest survive in trip_events.
INSERT INTO tracking.trip_stop_tasks (
    trip_stop_id, kind, subject_type, subject_id, status, done_at, done_by, proof
)
SELECT DISTINCT ON (ts.id, m.kind, le.rider_id)
    ts.id,
    m.kind,
    'passenger',
    le.rider_id,
    m.status,
    le.event_at,
    le.created_by,
    CASE WHEN le.notes IS NOT NULL THEN jsonb_build_object('notes', le.notes) END
FROM legacy_events le
INNER JOIN (VALUES
    ('check_in', 'pickup',  'done'),
    ('checkout', 'dropoff', 'done'),
    ('no_show',  'pickup',  'no_show')
) AS m(event_type, kind, status) ON m.event_type = le.event_type
INNER JOIN tracking.trips t
        ON t.route_id = le.route_id AND t.service_date = le.service_date AND t.route_schedule_id IS NULL
LEFT JOIN tracking.rider_assignments ra
       ON ra.rider_id = le.rider_id AND ra.route_id = le.route_id AND ra.status = 'active'
INNER JOIN LATERAL (
    SELECT s.id
    FROM tracking.trip_stops s
    WHERE s.trip_id = t.id
    ORDER BY
        s.stop_place_id = le.stop_id DESC NULLS LAST,
        s.stop_place_id = CASE m.kind WHEN 'pickup' THEN ra.pickup_stop_id ELSE ra.dropoff_stop_id END DESC NULLS LAST,
        CASE m.kind WHEN 'pickup' THEN s.sequence ELSE -s.sequence END
    LIMIT 1
) ts ON true
ORDER BY ts.id, m.kind, le.rider_id, le.event_at DESC, le.created_at DESC;

-- A stop where something was done was reached.
UPDATE tracking.trip_stops s
SET status = 'arrived', arrived_at = d.first_done
FROM (
    SELECT k.trip_stop_id, MIN(k.done_at) AS first_done
    FROM tracking.trip_stop_tasks k
    WHERE k.status = 'done'
    GROUP BY k.trip_stop_id
) d
WHERE d.trip_stop_id = s.id;

INSERT INTO tracking.trip_events (trip_id, type, payload, created_at, created_by)
SELECT
    t.id,
    le.event_type,
    jsonb_build_object('ride_event', to_jsonb(re)),
    le.event_at,
    le.created_by
FROM legacy_events le
INNER JOIN tracking.ride_events re ON re.id = le.id
INNER JOIN tracking.trips t
        ON t.route_id = le.route_id AND t.service_date = le.service_date AND t.route_schedule_id IS NULL;

-- ===========================================================================
-- ride_events goes
-- ===========================================================================

DROP FUNCTION IF EXISTS tracking.sp_record_ride_event(UUID, UUID, UUID, VARCHAR, DECIMAL, DECIMAL, TEXT, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_record_ride_event(UUID, UUID, UUID, VARCHAR, DECIMAL, DECIMAL, TEXT, UUID);
DROP TABLE legacy_events;
DROP TABLE tracking.ride_events;

-- ===========================================================================
-- Routes read and write their zone (D2)
-- ===========================================================================

DROP FUNCTION IF EXISTS tracking.sp_update_route(UUID, UUID, VARCHAR, UUID, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_get_route(UUID, UUID, UUID);

-- A zone PostgreSQL does not know would make every planned_start of the route a runtime error, so it
-- is refused at the door. pg_timezone_names is a catalogue scan, paid once per edit of a route.
CREATE FUNCTION tracking.sp_update_route(
    p_tenant_id UUID,
    p_route_id UUID,
    p_route_name VARCHAR(255),
    p_vehicle_id UUID,
    p_is_active BOOLEAN,
    p_timezone VARCHAR(64) DEFAULT NULL
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
    updated_at TIMESTAMP,
    timezone VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_timezone IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pg_timezone_names tz WHERE tz.name = p_timezone
    ) THEN
        RAISE EXCEPTION 'route.timezone' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.routes r SET
        route_name = COALESCE(NULLIF(TRIM(p_route_name), ''), r.route_name),
        vehicle_id = p_vehicle_id,
        is_active = COALESCE(p_is_active, r.is_active),
        timezone = COALESCE(p_timezone, r.timezone),
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
        r.updated_at,
        CAST(r.timezone AS VARCHAR);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_route(UUID, UUID, VARCHAR, UUID, BOOLEAN, VARCHAR) IS
'Edits a route; p_timezone NULL keeps the stored zone and an unknown IANA name raises route.timezone (TRACK-008 D2). Trips already built keep the zone they copied';

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
    updated_at TIMESTAMP,
    timezone VARCHAR
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
        r.updated_at,
        CAST(r.timezone AS VARCHAR)
    FROM tracking.routes r
    WHERE r.id = p_route_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_get_route(UUID, UUID, UUID) IS
'One route with its zone (TRACK-008 D2), scoped to p_scope_user_id''s organizations when it is set';
