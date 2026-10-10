CREATE OR REPLACE FUNCTION tracking.sp_update_calendar(p_tenant_id uuid, p_calendar_id uuid, p_name character varying, p_organization_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, organization_id uuid, name character varying, dates_count integer, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_organization_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.calendars c SET
        name            = COALESCE(NULLIF(TRIM(p_name), ''), c.name),
        organization_id = COALESCE(p_organization_id, c.organization_id),
        updated_at      = CURRENT_TIMESTAMP
    WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        c.id, c.tenant_id, c.organization_id, CAST(c.name AS VARCHAR),
        CAST((SELECT COUNT(*) FROM tracking.calendar_dates cd WHERE cd.calendar_id = c.id) AS INT),
        c.created_at, c.updated_at
    FROM tracking.calendars c
    WHERE c.id = p_calendar_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_calendar(uuid,uuid,character varying,uuid) IS 'Renames a calendar or moves it to another organization; a NULL argument keeps the stored value';
