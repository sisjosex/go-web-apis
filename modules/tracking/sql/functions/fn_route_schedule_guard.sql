CREATE OR REPLACE FUNCTION tracking.fn_route_schedule_guard(p_tenant_id uuid, p_route_id uuid, p_start_time time without time zone, p_valid_from date, p_valid_until date, p_calendar_id uuid, p_exclude_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Before the overlap test: daterange() itself raises on an inverted range, with Postgres's words.
    IF p_valid_until < p_valid_from THEN
        RAISE EXCEPTION 'schedule.range' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_calendar_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.route_schedules s
        WHERE s.route_id = p_route_id
          AND s.start_time = p_start_time
          AND (p_exclude_id IS NULL OR s.id <> p_exclude_id)
          AND daterange(s.valid_from, s.valid_until, '[]')
              && daterange(p_valid_from, p_valid_until, '[]')
    ) THEN
        RAISE EXCEPTION 'schedule.overlap' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_schedule_guard(uuid,uuid,time without time zone,date,date,uuid,uuid) IS 'The checks every schedule write shares: an end on or after the start (TRACK-029), route and calendar in the tenant, and no other schedule of the route already in force at the same time of day over the same dates';
