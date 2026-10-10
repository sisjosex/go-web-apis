CREATE OR REPLACE FUNCTION tracking.sp_list_route_schedules(p_tenant_id uuid, p_route_id uuid)
 RETURNS TABLE(id uuid, route_id uuid, days_of_week smallint, start_time character varying, valid_from date, valid_until date, calendar_id uuid, calendar_name character varying, created_at timestamp without time zone, updated_at timestamp without time zone, arrival_time character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        s.id,
        s.route_id,
        s.days_of_week,
        CAST(TO_CHAR(s.start_time, 'HH24:MI') AS VARCHAR),
        s.valid_from,
        s.valid_until,
        s.calendar_id,
        CAST(c.name AS VARCHAR),
        s.created_at,
        s.updated_at,
        -- TRACK-055 D2: the arrival the estimate keeps, while it owns the departure.
        CAST(TO_CHAR(s.start_time + make_interval(mins => s.departure_auto_min), 'HH24:MI') AS VARCHAR)
    FROM tracking.route_schedules s
    LEFT JOIN tracking.calendars c ON c.id = s.calendar_id
    WHERE s.route_id = p_route_id
    -- The schedule an operator opens the screen to change is the one in force, and two schedules of
    -- the same day are read in departure order.
    ORDER BY s.valid_from DESC, s.start_time ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_route_schedules(uuid,uuid) IS 'A route''s schedules, newest validity first; raises tracking.route.not-found outside the tenant';
