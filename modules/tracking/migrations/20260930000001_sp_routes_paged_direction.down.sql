-- Reverses TRACK-002 step 1: the route SPs return to their unpaged, name-less shape and
-- sp_get_rider_status again requires a legacy end time.

DROP FUNCTION IF EXISTS tracking.sp_update_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, DECIMAL, DECIMAL, INT, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_create_route(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, DECIMAL, DECIMAL, INT);
DROP FUNCTION IF EXISTS tracking.fn_route_company_check(UUID, UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_route(UUID, UUID, UUID);

DROP INDEX IF EXISTS tracking.idx_routes_code_trgm;
DROP INDEX IF EXISTS tracking.idx_routes_name_trgm;
DROP INDEX IF EXISTS tracking.idx_routes_company_name;

CREATE OR REPLACE FUNCTION tracking.sp_list_routes(p_tenant_id uuid, p_company_id uuid, p_is_active boolean, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, company_id uuid, vehicle_id uuid, route_name character varying, route_code character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, schedule_type character varying, scheduled_start_time time without time zone, scheduled_end_time time without time zone, estimated_duration_minutes integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
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
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_is_active IS NULL OR r.is_active = p_is_active)
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.rider_route_assignments ra
          INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
          WHERE ra.route_id = r.id
            AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
            AND rr.organization_id = ANY(v_scope)
      ))
    ORDER BY r.created_at DESC;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_get_route(p_tenant_id uuid, p_route_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, company_id uuid, vehicle_id uuid, route_name character varying, route_code character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, schedule_type character varying, scheduled_start_time time without time zone, scheduled_end_time time without time zone, estimated_duration_minutes integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, timezone character varying)
 LANGUAGE plpgsql
AS $function$
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
              FROM tracking.rider_route_assignments ra
              INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
              WHERE ra.route_id = r.id
                AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
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
$function$;


COMMENT ON FUNCTION tracking.sp_get_route(UUID, UUID, UUID) IS
'One route with its zone (TRACK-008 D2), scoped to p_scope_user_id''s organizations when it is set';

CREATE OR REPLACE FUNCTION tracking.sp_create_route(
    p_tenant_id UUID,
    p_company_id UUID,
    p_route_name VARCHAR(255),
    p_origin_address VARCHAR(500),
    p_destination_address VARCHAR(500),
    p_route_code VARCHAR(50),
    p_vehicle_id UUID,
    p_origin_lat DECIMAL(10,8),
    p_origin_lng DECIMAL(11,8),
    p_destination_lat DECIMAL(10,8),
    p_destination_lng DECIMAL(11,8),
    p_schedule_type VARCHAR(50),
    p_scheduled_start_time TIME,
    p_scheduled_end_time TIME,
    p_estimated_duration_minutes INT
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
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_route_code IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.routes r
        WHERE r.company_id = p_company_id AND r.route_code = TRIM(p_route_code)
    ) THEN
        RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.routes (
        company_id, vehicle_id, route_name, route_code,
        origin_address, origin_lat, origin_lng,
        destination_address, destination_lat, destination_lng,
        schedule_type, scheduled_start_time, scheduled_end_time,
        estimated_duration_minutes, is_active
    )
    VALUES (
        p_company_id,
        p_vehicle_id,
        TRIM(p_route_name),
        NULLIF(TRIM(p_route_code), ''),
        TRIM(p_origin_address),
        p_origin_lat,
        p_origin_lng,
        TRIM(p_destination_address),
        p_destination_lat,
        p_destination_lng,
        COALESCE(NULLIF(TRIM(p_schedule_type), ''), 'custom'),
        p_scheduled_start_time,
        p_scheduled_end_time,
        p_estimated_duration_minutes,
        true
    )
    RETURNING
        tracking.routes.id,
        tracking.routes.company_id,
        tracking.routes.vehicle_id,
        CAST(tracking.routes.route_name AS VARCHAR),
        CAST(tracking.routes.route_code AS VARCHAR),
        CAST(tracking.routes.origin_address AS VARCHAR),
        tracking.routes.origin_lat,
        tracking.routes.origin_lng,
        CAST(tracking.routes.destination_address AS VARCHAR),
        tracking.routes.destination_lat,
        tracking.routes.destination_lng,
        CAST(tracking.routes.schedule_type AS VARCHAR),
        tracking.routes.scheduled_start_time,
        tracking.routes.scheduled_end_time,
        tracking.routes.estimated_duration_minutes,
        tracking.routes.is_active,
        tracking.routes.created_at,
        tracking.routes.updated_at;
END;
$$ LANGUAGE plpgsql;

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

CREATE OR REPLACE FUNCTION tracking.sp_get_rider_status(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, driver_name character varying, vehicle_latitude numeric, vehicle_longitude numeric, vehicle_speed numeric, location_age_seconds integer, last_event_type character varying, last_event_time timestamp without time zone, last_event_notes text, last_event_stop character varying, scheduled_pickup_stop character varying, scheduled_dropoff_stop character varying, active_alerts integer)
 LANGUAGE plpgsql
AS $function$
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
        CAST(last_event.done_at AS TIMESTAMP) AS last_event_time,
        CAST(last_event.notes AS TEXT) AS last_event_notes,
        CAST(last_event.stop_name AS VARCHAR) AS last_event_stop,
        CAST(COALESCE(trip_stop.pickup_name, pickup_stop.name) AS VARCHAR) AS scheduled_pickup_stop,
        CAST(COALESCE(trip_stop.dropoff_name, dropoff_stop.name) AS VARCHAR) AS scheduled_dropoff_stop,
        COALESCE((
            SELECT COUNT(*)::INT
            FROM tracking.route_alerts alerts
            WHERE alerts.route_id = ra.route_id
              AND alerts.status = 'active'
        ), 0) AS active_alerts
    FROM tracking.riders rider
    -- Which route is the rider's now: of the assignments in force on each route's local today, the
    -- first route that has not ended.
    LEFT JOIN LATERAL (
        SELECT ra_inner.route_id,
               ra_inner.pickup_stop_place_id AS pickup_stop_id,
               ra_inner.dropoff_stop_place_id AS dropoff_stop_id
        FROM tracking.rider_route_assignments ra_inner
        INNER JOIN tracking.routes ro ON ro.id = ra_inner.route_id
        WHERE ra_inner.rider_id = rider.id
          AND tracking.fn_assignment_on(ra_inner.days_of_week, ra_inner.valid_from, ra_inner.valid_until,
                                        CAST(now() AT TIME ZONE ro.timezone AS DATE))
          AND ro.scheduled_end_time >= LOCALTIME
        ORDER BY ro.scheduled_start_time, ro.id
        LIMIT 1
    ) ra ON true
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.trips t ON t.id = tracking.fn_current_trip(p_tenant_id, ra.route_id)
    LEFT JOIN tracking.vehicles v ON v.id = COALESCE(t.vehicle_id, route.vehicle_id)
    LEFT JOIN tracking.drivers d ON d.id = COALESCE(t.driver_id, route.default_driver_id)
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    -- The rider's latest transition on any trip, read back in the words ride_events used.
    LEFT JOIN LATERAL (
        SELECT
            CASE
                WHEN k.status = 'no_show' THEN 'no_show'
                WHEN k.kind = 'pickup' THEN 'check_in'
                ELSE 'checkout'
            END AS event_type,
            k.done_at,
            k.proof->>'notes' AS notes,
            sp.name AS stop_name
        FROM tracking.trip_stop_tasks k
        INNER JOIN tracking.trip_stops ts ON ts.id = k.trip_stop_id
        INNER JOIN tracking.trips kt ON kt.id = ts.trip_id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE k.subject_id = p_rider_id
          AND k.subject_type = 'passenger'
          AND k.done_at IS NOT NULL
          AND k.status IN ('done', 'no_show')
          AND kt.tenant_id = p_tenant_id
        ORDER BY k.done_at DESC
        LIMIT 1
    ) last_event ON true
    -- Where the current trip picks the rider up and drops them off; the assignment's stops when the
    -- route has no trip today.
    LEFT JOIN LATERAL (
        SELECT
            MAX(sp.name) FILTER (WHERE k.kind = 'pickup') AS pickup_name,
            MAX(sp.name) FILTER (WHERE k.kind = 'dropoff') AS dropoff_name
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE ts.trip_id = t.id
          AND k.subject_type = 'passenger'
          AND k.subject_id = p_rider_id
    ) trip_stop ON true
    LEFT JOIN tracking.stop_places pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.stop_places dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$function$;
