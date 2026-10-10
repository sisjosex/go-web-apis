CREATE OR REPLACE FUNCTION tracking.sp_list_calendar_dates(p_tenant_id uuid, p_calendar_id uuid, p_from date, p_to date)
 RETURNS TABLE(date date, kind character varying, label character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR)
    FROM tracking.calendar_dates cd
    WHERE cd.calendar_id = p_calendar_id
      AND (p_from IS NULL OR cd.date >= p_from)
      AND (p_to   IS NULL OR cd.date <= p_to)
    ORDER BY cd.date ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_calendar_dates(uuid,uuid,date,date) IS 'A calendar''s dates, oldest first, optionally narrowed to a window; raises tracking.calendar.not-found outside the tenant';
