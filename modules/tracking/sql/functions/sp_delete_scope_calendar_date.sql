CREATE OR REPLACE FUNCTION tracking.sp_delete_scope_calendar_date(p_tenant_id uuid, p_calendar_id uuid, p_date date)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_deleted INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id AND c.scope IN ('tenant', 'organization')
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.calendar_dates cd
    WHERE cd.calendar_id = p_calendar_id AND cd.date = p_date;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;

    IF v_deleted > 0 THEN
        PERFORM tracking.fn_route_changed(cr.route_id, p_date, p_date)
        FROM tracking.fn_calendar_routes(p_tenant_id, p_calendar_id) cr;
    END IF;
    RETURN v_deleted > 0;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_scope_calendar_date(uuid,uuid,date) IS 'Gives one day of the tenant calendar or a client calendar back to the routes it reaches, writing their route.changed; false when the day was not marked; raises tracking.calendar.not-found for a schedule calendar or another tenant''s (TRACK-076 D1)';
