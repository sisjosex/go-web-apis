CREATE OR REPLACE FUNCTION tracking.fn_route_schedule_row(p_tenant_id uuid, p_schedule_id uuid)
 RETURNS TABLE(id uuid, route_id uuid, days_of_week smallint, start_time character varying, valid_from date, valid_until date, calendar_id uuid, calendar_name character varying, created_at timestamp without time zone, updated_at timestamp without time zone, arrival_time character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
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
    LEFT JOIN tracking.calendars c ON c.id = s.calendar_id AND c.tenant_id = p_tenant_id
    WHERE s.id = p_schedule_id;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_schedule_row(uuid,uuid) IS 'One schedule in the shape every schedule endpoint returns, with the name of the calendar it reads';
