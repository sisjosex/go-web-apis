CREATE OR REPLACE FUNCTION tracking.sp_create_route_version(p_tenant_id uuid, p_route_id uuid, p_effective_from date, p_stops jsonb, p_created_by uuid)
 RETURNS TABLE(id uuid, route_id uuid, effective_from date, effective_to date, stops_count integer, created_by uuid, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_latest     DATE;
    v_version_id UUID;
    v_stops      INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT MAX(rv.effective_from) INTO v_latest
    FROM tracking.route_versions rv
    WHERE rv.route_id = p_route_id;

    IF v_latest IS NOT NULL AND p_effective_from <= v_latest THEN
        RAISE EXCEPTION 'route.version-overlap' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.route_versions rv
       SET effective_to = p_effective_from - 1,
           updated_at   = CURRENT_TIMESTAMP
     WHERE rv.route_id = p_route_id
       AND rv.effective_to IS NULL;

    INSERT INTO tracking.route_versions (route_id, effective_from, effective_to, created_by)
    VALUES (p_route_id, p_effective_from, NULL, p_created_by)
    RETURNING tracking.route_versions.id INTO v_version_id;

    v_stops := tracking.fn_route_version_stops_json(p_tenant_id, v_version_id, p_stops);

    -- The route runs a different list from p_effective_from onwards, with no end: date_to NULL is
    -- "and every day after". Written here so the row exists if and only if the publish committed.
    INSERT INTO tracking.outbox (topic, payload)
    VALUES ('route.changed', jsonb_build_object(
        'route_id',  p_route_id,
        'date_from', p_effective_from,
        'date_to',   NULL
    ));

    RETURN QUERY
    SELECT
        rv.id,
        rv.route_id,
        rv.effective_from,
        rv.effective_to,
        v_stops,
        rv.created_by,
        rv.created_at,
        rv.updated_at
    FROM tracking.route_versions rv
    WHERE rv.id = v_version_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_route_version(uuid,uuid,date,jsonb,uuid) IS 'Publishes a route''s next stop list from p_effective_from, closing the one in force the day before; raises tracking.route.version-overlap when it would not start after the latest';
