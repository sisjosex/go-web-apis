CREATE OR REPLACE FUNCTION tracking.sp_create_calendar(p_tenant_id uuid, p_name character varying, p_organization_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, organization_id uuid, name character varying, dates_count integer, created_at timestamp without time zone, updated_at timestamp without time zone)
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

    INSERT INTO tracking.calendars (tenant_id, organization_id, name)
    VALUES (p_tenant_id, p_organization_id, TRIM(p_name))
    RETURNING tracking.calendars.id INTO v_id;

    RETURN QUERY
    SELECT c.id, c.tenant_id, c.organization_id, CAST(c.name AS VARCHAR), CAST(0 AS INT),
           c.created_at, c.updated_at
    FROM tracking.calendars c
    WHERE c.id = v_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_calendar(uuid,character varying,uuid) IS 'Creates a calendar in a tenant; raises tracking.organization.not-found when the organization it belongs to is not the tenant''s';
