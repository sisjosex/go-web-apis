CREATE OR REPLACE FUNCTION tracking.sp_get_route_realtime_status(p_tenant_id uuid, p_route_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, driver_name character varying, current_latitude numeric, current_longitude numeric, current_speed numeric, location_age_seconds integer, total_riders integer, boarded_count integer, arrived_count integer, no_show_count integer, pending_count integer, active_alerts integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_trip UUID;
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

    v_trip := tracking.fn_current_trip(p_tenant_id, p_route_id);

    RETURN QUERY
    SELECT
        r.id AS route_id,
        CAST(r.route_name AS VARCHAR),
        v.id AS vehicle_id,
        CAST(v.plate_number AS VARCHAR) AS license_plate,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR) AS driver_name,
        vl.latitude AS current_latitude,
        vl.longitude AS current_longitude,
        vl.speed AS current_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        CAST(COALESCE(tk.total, ra.total, 0) AS INT) AS total_riders,
        CAST(COALESCE(tk.boarded, 0) AS INT) AS boarded_count,
        CAST(COALESCE(tk.arrived, 0) AS INT) AS arrived_count,
        CAST(COALESCE(tk.no_show, 0) AS INT) AS no_show_count,
        CAST(COALESCE(tk.total - tk.touched, ra.total, 0) AS INT) AS pending_count,
        COALESCE(( SELECT COUNT(*)::INT FROM tracking.route_alerts alerts WHERE alerts.route_id = r.id AND alerts.status = 'active' ), 0) AS active_alerts
    FROM tracking.routes r
    LEFT JOIN tracking.trips t ON t.id = v_trip
    LEFT JOIN tracking.vehicles v ON v.id = COALESCE(t.vehicle_id, r.vehicle_id)
    LEFT JOIN tracking.drivers d ON d.id = COALESCE(t.driver_id, r.default_driver_id)
    LEFT JOIN LATERAL (
        SELECT CAST(ST_Y(lp.location::geometry) AS NUMERIC) AS latitude,
               CAST(ST_X(lp.location::geometry) AS NUMERIC) AS longitude,
               CAST(lp.speed AS NUMERIC) AS speed,
               CAST(lp.heading AS NUMERIC) AS heading,
               lp.recorded_at
        FROM tracking.vehicle_last_position lp
        WHERE lp.vehicle_id = v.id
    ) vl ON true
    -- The trip's roster: every rider holding a task that is not cancelled, and what happened to them.
    LEFT JOIN LATERAL (
        SELECT
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status <> 'cancelled') AS total,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'done') AS boarded,
            COUNT(*) FILTER (WHERE k.kind = 'dropoff' AND k.status = 'done') AS arrived,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'no_show') AS no_show,
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status IN ('done', 'no_show')) AS touched
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE ts.trip_id = t.id
        HAVING t.id IS NOT NULL
    ) tk ON true
    -- No trip today: the riders assigned for the route's local today, none of whom has been picked up.
    LEFT JOIN LATERAL (
        SELECT COUNT(*) AS total
        FROM tracking.rider_route_assignments ra_inner
        WHERE ra_inner.route_id = r.id
          AND tracking.fn_assignment_on(ra_inner.days_of_week, ra_inner.valid_from, ra_inner.valid_until,
                                        CAST(now() AT TIME ZONE r.timezone AS DATE))
          AND t.id IS NULL
    ) ra ON true
    WHERE r.id = p_route_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_route_realtime_status(uuid,uuid,uuid) IS 'A route''s live status over today''s current trip (TRACK-008 D3): rider counts from that trip''s tasks, vehicle and driver from the trip or the route''s defaults; scoped to p_scope_user_id''s organizations when it is set';
