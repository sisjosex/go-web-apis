CREATE OR REPLACE FUNCTION tracking.sp_create_stop_place(p_tenant_id uuid, p_name character varying, p_address character varying, p_latitude double precision, p_longitude double precision, p_organization_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, organization_id uuid, name character varying, address character varying, latitude numeric, longitude numeric, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id UUID;
BEGIN
    IF p_organization_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.stop_places (tenant_id, organization_id, name, address, location)
    VALUES (
        p_tenant_id,
        p_organization_id,
        TRIM(p_name),
        NULLIF(TRIM(p_address), ''),
        ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography
    )
    RETURNING tracking.stop_places.id INTO v_id;

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
    WHERE sp.id = v_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_stop_place(uuid,character varying,character varying,double precision,double precision,uuid) IS 'Creates a stop place in a tenant; raises tracking.organization.not-found when the organization it is pinned to is not the tenant''s';
