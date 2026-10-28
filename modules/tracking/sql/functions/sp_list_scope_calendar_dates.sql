CREATE OR REPLACE FUNCTION tracking.sp_list_scope_calendar_dates(p_tenant_id uuid, p_from date, p_to date)
 RETURNS TABLE(date date, kind character varying, label character varying, calendar_id uuid, calendar_name character varying, scope character varying, organization_id uuid, organization_name character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    -- The month page shows the dates that reach routes without a schedule naming them: the tenant
    -- calendar and the client calendars (TRACK-076 D1). A schedule calendar stays on its own page.
    RETURN QUERY
    SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR),
           c.id, CAST(c.name AS VARCHAR), CAST(c.scope AS VARCHAR),
           c.organization_id, CAST(o.name AS VARCHAR)
    FROM tracking.calendars c
    INNER JOIN tracking.calendar_dates cd ON cd.calendar_id = c.id
    LEFT JOIN tracking.organizations o ON o.id = c.organization_id
    WHERE c.tenant_id = p_tenant_id
      AND c.scope IN ('tenant', 'organization')
      AND (p_from IS NULL OR cd.date >= p_from)
      AND (p_to   IS NULL OR cd.date <= p_to)
    ORDER BY cd.date ASC, c.scope DESC, o.name ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_scope_calendar_dates(uuid,date,date) IS 'Every date of the tenant calendar and the client calendars in a window, oldest first, with the calendar, its scope and the client it belongs to (TRACK-076 D1)';
