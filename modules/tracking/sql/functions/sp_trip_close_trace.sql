CREATE OR REPLACE FUNCTION tracking.sp_trip_close_trace(p_tenant_id uuid, p_trip_id uuid, p_polyline text, p_distance_km numeric, p_source character varying)
 RETURNS void
 LANGUAGE sql
AS $function$
    UPDATE tracking.trips t
    SET polyline = p_polyline,
        distance_km = ROUND(p_distance_km, 2),
        trace_source = p_source,
        updated_at = now()
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_close_trace(uuid,uuid,text,numeric,character varying) IS 'Stores a trip''s driven path and its length in km, and where the line came from (TRACK-010)';
