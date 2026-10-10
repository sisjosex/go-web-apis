CREATE OR REPLACE FUNCTION tracking.fn_route_alert_rows(p_tenant_id uuid, p_ids uuid[])
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, trip_id uuid, alert_type character varying, title character varying, message text, severity character varying, estimated_delay_minutes integer, status character varying, lat double precision, lng double precision, created_by uuid, created_by_name character varying, created_at timestamp with time zone, resolved_at timestamp with time zone, resolved_by uuid, resolved_by_name character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        a.id,
        a.route_id,
        CAST(r.route_name AS VARCHAR),
        a.vehicle_id,
        CAST(v.plate_number AS VARCHAR),
        a.trip_id,
        CAST(a.alert_type AS VARCHAR),
        CAST(COALESCE(a.title, a.alert_type) AS VARCHAR),
        a.message,
        CAST(a.severity AS VARCHAR),
        a.estimated_delay_minutes,
        CAST(a.status AS VARCHAR),
        ST_Y(CAST(a.location AS GEOMETRY)),
        ST_X(CAST(a.location AS GEOMETRY)),
        a.created_by,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', cu.first_name, cu.last_name), ''), cu.email) AS VARCHAR),
        CAST(a.created_at AS TIMESTAMPTZ),
        CAST(a.resolved_at AS TIMESTAMPTZ),
        a.resolved_by,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', ru.first_name, ru.last_name), ''), ru.email) AS VARCHAR)
    FROM tracking.route_alerts a
    INNER JOIN tracking.routes r ON r.id = a.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = a.vehicle_id
    LEFT JOIN auth.users cu ON cu.id = a.created_by AND cu.deleted_at IS NULL
    LEFT JOIN auth.users ru ON ru.id = a.resolved_by AND ru.deleted_at IS NULL
    WHERE tc.tenant_id = p_tenant_id
      AND (p_ids IS NULL OR a.id = ANY(p_ids));
$function$;

COMMENT ON FUNCTION tracking.fn_route_alert_rows(uuid,uuid[]) IS 'The row shape of the tenant''s alerts (all when p_ids is NULL): route_alerts columns plus route_name, license_plate, lat/lng and the raiser''s and resolver''s names (TRACK-004)';
