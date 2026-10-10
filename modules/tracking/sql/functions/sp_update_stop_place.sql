CREATE OR REPLACE FUNCTION tracking.sp_update_stop_place(p_tenant_id uuid, p_stop_place_id uuid, p_name character varying, p_address character varying, p_latitude double precision, p_longitude double precision, p_organization_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, organization_id uuid, name character varying, address character varying, latitude numeric, longitude numeric, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.stop_places sp
        WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_organization_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.stop_places sp SET
        name            = COALESCE(NULLIF(TRIM(p_name), ''), sp.name),
        address         = COALESCE(NULLIF(TRIM(p_address), ''), sp.address),
        organization_id = COALESCE(p_organization_id, sp.organization_id),
        location        = CASE
            WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography
            ELSE sp.location
        END,
        updated_at = CURRENT_TIMESTAMP
    WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id;

    -- Moving or renaming a place changes every route that calls there, from today on: one row per
    -- affected route, in this transaction (INFRA-001 D3). A place no version names changes nothing,
    -- so it writes nothing.
    INSERT INTO tracking.outbox (topic, payload)
    SELECT DISTINCT
        'route.changed',
        jsonb_build_object(
            'route_id',  rv.route_id,
            'date_from', CURRENT_DATE,
            'date_to',   CURRENT_DATE
        )
    FROM tracking.route_version_stops vs
    INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
    WHERE vs.stop_place_id = p_stop_place_id;

    RETURN QUERY
    SELECT
        sp.id,
        sp.tenant_id,
        sp.organization_id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        sp.created_at,
        sp.updated_at
    FROM tracking.stop_places sp
    WHERE sp.id = p_stop_place_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_stop_place(uuid,uuid,character varying,character varying,double precision,double precision,uuid) IS 'Edits a stop place; a NULL argument keeps the stored value and the coordinate moves only when both halves are sent';
