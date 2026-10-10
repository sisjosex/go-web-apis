CREATE OR REPLACE FUNCTION tracking.sp_vehicle_schedule_conflicts(p_tenant_id uuid, p_vehicle_id uuid, p_route_id uuid DEFAULT NULL::uuid, p_driver_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(route_id uuid, route_name character varying, reason character varying, days_of_week smallint, start_time character varying, end_time character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH runs AS (
        SELECT
            r.id AS route_id,
            r.route_name,
            r.vehicle_id,
            COALESCE(r.default_driver_id, v.default_driver_id) AS driver_id,
            s.days_of_week,
            s.start_time,
            s.start_time + make_interval(mins => COALESCE(r.estimated_duration_minutes, 60)) AS end_time,
            s.valid_from,
            COALESCE(s.valid_until, DATE '9999-12-31') AS valid_until
        FROM tracking.route_schedules s
        INNER JOIN tracking.routes r ON r.id = s.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id AND tc.tenant_id = p_tenant_id
        LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
        WHERE r.is_active
          AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
    ),
    mine AS (
        SELECT * FROM runs ru
        WHERE (p_route_id IS NOT NULL AND ru.route_id = p_route_id)
           OR (p_route_id IS NULL AND ru.vehicle_id = p_vehicle_id)
    )
    SELECT DISTINCT
        o.route_id,
        CAST(o.route_name AS VARCHAR),
        CAST(CASE WHEN o.vehicle_id = p_vehicle_id THEN 'vehicle' ELSE 'driver' END AS VARCHAR),
        CAST(o.days_of_week & m.days_of_week AS SMALLINT),
        CAST(TO_CHAR(o.start_time, 'HH24:MI') AS VARCHAR),
        CAST(TO_CHAR(o.end_time, 'HH24:MI') AS VARCHAR)
    FROM mine m
    INNER JOIN runs o
            ON o.route_id NOT IN (SELECT mi.route_id FROM mine mi)
           AND (o.vehicle_id = p_vehicle_id OR (p_driver_id IS NOT NULL AND o.driver_id = p_driver_id))
           AND (o.days_of_week & m.days_of_week) <> 0
           AND o.start_time < m.end_time
           AND m.start_time < o.end_time
           AND o.valid_from <= m.valid_until
           AND m.valid_from <= o.valid_until
    ORDER BY 5, 2;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_vehicle_schedule_conflicts(uuid,uuid,uuid,uuid) IS 'The runs that would overlap if p_route_id (or, NULL, the vehicle''s own routes) ran on p_vehicle_id: other routes of the vehicle, and with p_driver_id the routes that driver already drives; reason vehicle|driver, the shared weekdays and the other run''s hours. Raises vehicle.not-found (TRACK-034 D2)';
