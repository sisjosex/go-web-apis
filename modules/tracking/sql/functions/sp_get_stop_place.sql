CREATE OR REPLACE FUNCTION tracking.sp_get_stop_place(p_tenant_id uuid, p_stop_place_id uuid)
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
    WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id;
END;
$function$;
