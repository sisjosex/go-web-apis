CREATE OR REPLACE FUNCTION tracking.fn_trip_rows(p_tenant_id uuid, p_ids uuid[])
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, service_block jsonb)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        t.id,
        t.route_id,
        CAST(r.route_name AS VARCHAR),
        CAST(r.direction AS VARCHAR),
        t.route_schedule_id,
        t.service_date,
        CAST(t.timezone AS VARCHAR),
        t.planned_start,
        t.vehicle_id,
        CAST(v.plate_number AS VARCHAR),
        t.driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(t.status AS VARCHAR),
        t.started_at,
        t.ended_at,
        t.is_overridden,
        CAST(pr.stops AS INT),
        CAST(pr.total AS INT),
        CAST(pr.done AS INT),
        t.created_at,
        t.updated_at,
        tracking.fn_service_block(t.tenant_id, t.vehicle_id, t.driver_id)
    FROM tracking.trips t
    INNER JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.vehicles v ON v.id = t.vehicle_id
    LEFT JOIN tracking.drivers d ON d.id = t.driver_id
    CROSS JOIN LATERAL (
        SELECT
            COUNT(DISTINCT ts.id) AS stops,
            COUNT(k.id) FILTER (WHERE k.status <> 'cancelled') AS total,
            COUNT(k.id) FILTER (WHERE k.status IN ('done', 'no_show')) AS done
        FROM tracking.trip_stops ts
        LEFT JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE ts.trip_id = t.id
    ) pr
    WHERE t.tenant_id = p_tenant_id
      AND (p_ids IS NULL OR t.id = ANY(p_ids));
$function$;

COMMENT ON FUNCTION tracking.fn_trip_rows(uuid,uuid[]) IS 'The one row shape of a trip — route, plate and driver names, stops_count, tasks_total (not cancelled) and tasks_done (done or no_show), service_block { subject, reason } of a blocked vehicle or driver (TRACK-064) — for p_ids, or every trip of the tenant when NULL (TRACK-020)';
