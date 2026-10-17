CREATE OR REPLACE FUNCTION tracking.fn_trip_stop_redirects(p_tenant_id uuid, p_route_id uuid, p_version_id uuid, p_service_date date)
 RETURNS TABLE(stop_place_id uuid, pickup_stop_place_id uuid, dropoff_stop_place_id uuid)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    -- TRACK-065 D3: a rider whose stop the day leaves out boards at the next stop the day still calls
    -- at and gets off at the previous one, both read in the version's order.
    RETURN QUERY
    WITH plan AS (
        SELECT p.stop_place_id FROM tracking.fn_trip_stop_plan(p_tenant_id, p_route_id, p_version_id, p_service_date) p
    ),
    version AS (
        SELECT vs.stop_place_id, vs.sequence,
               EXISTS (SELECT 1 FROM plan pl WHERE pl.stop_place_id = vs.stop_place_id) AS kept
        FROM tracking.route_version_stops vs
        WHERE vs.version_id = p_version_id
    )
    SELECT
        v.stop_place_id,
        (SELECT k.stop_place_id FROM version k WHERE k.kept AND k.sequence > v.sequence ORDER BY k.sequence LIMIT 1),
        (SELECT k.stop_place_id FROM version k WHERE k.kept AND k.sequence < v.sequence ORDER BY k.sequence DESC LIMIT 1)
    FROM version v
    WHERE NOT v.kept;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_trip_stop_redirects(uuid,uuid,uuid,date) IS 'For every stop of the version that one date''s plan (fn_trip_stop_plan) leaves out, where its riders board instead (the next stop still called at) and get off (the previous one); NULL when there is none (TRACK-065 D3)';
