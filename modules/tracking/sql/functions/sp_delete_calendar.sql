CREATE OR REPLACE FUNCTION tracking.sp_delete_calendar(p_tenant_id uuid, p_calendar_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_deleted INT;
    v_from    DATE;
    v_to      DATE;
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

    -- A tenant or client calendar reaches routes without a schedule pointing at it (TRACK-076 D1):
    -- its days come back to them.
    SELECT MIN(cd.date), MAX(cd.date) INTO v_from, v_to
    FROM tracking.calendar_dates cd WHERE cd.calendar_id = p_calendar_id;
    IF v_from IS NOT NULL THEN
        PERFORM tracking.fn_route_changed(cr.route_id, v_from, v_to)
        FROM tracking.fn_calendar_routes(p_tenant_id, p_calendar_id) cr;
    END IF;

    DELETE FROM tracking.calendars c
    WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted > 0;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_calendar(uuid,uuid) IS 'Deletes a calendar; raises tracking.calendar.in-use while any schedule still points at it; a tenant or client calendar gives its days back to the routes it reached (TRACK-076 D1)';
