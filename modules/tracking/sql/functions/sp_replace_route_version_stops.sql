CREATE OR REPLACE FUNCTION tracking.sp_replace_route_version_stops(p_tenant_id uuid, p_version_id uuid, p_stops jsonb)
 RETURNS TABLE(id uuid, route_id uuid, version_id uuid, stop_place_id uuid, stop_name character varying, address character varying, latitude numeric, longitude numeric, sequence integer, planned_offset_min integer, dwell_sec integer, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_effective_from DATE;
BEGIN
    SELECT rv.effective_from INTO v_effective_from
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.id = p_version_id AND tc.tenant_id = p_tenant_id;

    IF v_effective_from IS NULL THEN
        RAISE EXCEPTION 'route.version-not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_effective_from < CURRENT_DATE THEN
        RAISE EXCEPTION 'route.version-closed' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.route_version_stops vs WHERE vs.version_id = p_version_id;
    PERFORM tracking.fn_route_version_stops_json(p_tenant_id, p_version_id, p_stops);

    UPDATE tracking.route_versions rv
       SET updated_at = CURRENT_TIMESTAMP
     WHERE rv.id = p_version_id;

    INSERT INTO tracking.outbox (topic, payload)
    SELECT 'route.changed', jsonb_build_object(
        'route_id',  rv.route_id,
        'date_from', rv.effective_from,
        'date_to',   NULL
    )
    FROM tracking.route_versions rv
    WHERE rv.id = p_version_id;

    RETURN QUERY
    SELECT
        vs.id,
        rv.route_id,
        rv.id,
        sp.id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        vs.sequence,
        vs.planned_offset_min,
        vs.dwell_sec,
        vs.created_at,
        vs.updated_at
    FROM tracking.route_version_stops vs
    INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
    INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id
    WHERE vs.version_id = p_version_id
    ORDER BY vs.sequence ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_replace_route_version_stops(uuid,uuid,jsonb) IS 'Replaces a route version''s whole stop list in one statement and returns it; raises tracking.route.version-closed once the version''s first day has passed';
