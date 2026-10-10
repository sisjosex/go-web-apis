CREATE OR REPLACE FUNCTION tracking.sp_get_calendar(p_tenant_id uuid, p_calendar_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, organization_id uuid, name character varying, dates_count integer, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        c.id,
        c.tenant_id,
        c.organization_id,
        CAST(c.name AS VARCHAR),
        CAST((SELECT COUNT(*) FROM tracking.calendar_dates cd WHERE cd.calendar_id = c.id) AS INT),
        c.created_at,
        c.updated_at
    FROM tracking.calendars c
    WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_calendar(uuid,uuid) IS 'One calendar of a tenant with how many dates it holds; raises tracking.calendar.not-found (TRACK-029)';
