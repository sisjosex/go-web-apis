CREATE OR REPLACE FUNCTION tracking.fn_stop_place_minutes(p_tenant_id uuid, p_from uuid, p_to uuid)
 RETURNS integer
 LANGUAGE sql
 STABLE
AS $function$
    -- The TRACK-038 estimate for one leg: straight line × 1.3 at 25 km/h, plus a minute for the stop.
    SELECT CAST(ROUND(COALESCE(ST_Distance(a.location, b.location), 0) * 1.3 / 1000 / 25 * 60) AS INT) + 1
    FROM tracking.stop_places a, tracking.stop_places b
    WHERE a.id = p_from AND a.tenant_id = p_tenant_id
      AND b.id = p_to AND b.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.fn_stop_place_minutes(uuid,uuid,uuid) IS 'Minutes from one stop place to another by the straight-line rule of TRACK-038 (km × 1.3 at 25 km/h + 1 min for the stop); 1 when either place has no location, NULL when either is not the tenant''s';
