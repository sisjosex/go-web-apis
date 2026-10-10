CREATE OR REPLACE FUNCTION tracking.sp_trip_trace_points(p_tenant_id uuid, p_trip_id uuid)
 RETURNS TABLE(lat double precision, lng double precision)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT ST_Y(p.location::geometry), ST_X(p.location::geometry)
    FROM tracking.trips t
    INNER JOIN tracking.vehicle_positions p
        ON p.trip_id = t.id
       AND p.recorded_at >= t.started_at
       AND p.recorded_at <= COALESCE(t.ended_at, now())
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id
      AND t.started_at IS NOT NULL
    ORDER BY p.recorded_at;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_trace_points(uuid,uuid) IS 'The GPS points of a trip between its start and end, oldest first (TRACK-010)';
