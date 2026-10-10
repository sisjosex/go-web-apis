CREATE OR REPLACE FUNCTION tracking.sp_delete_calendar(p_tenant_id uuid, p_calendar_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_deleted INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.route_schedules s WHERE s.calendar_id = p_calendar_id
    ) THEN
        RAISE EXCEPTION 'calendar.in-use' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.calendars c
    WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted > 0;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_calendar(uuid,uuid) IS 'Deletes a calendar; raises tracking.calendar.in-use while any schedule still points at it';
