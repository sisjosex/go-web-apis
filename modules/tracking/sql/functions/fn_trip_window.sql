CREATE OR REPLACE FUNCTION tracking.fn_trip_window(p_tenant_id uuid, p_route_id uuid, p_from date, p_to date)
 RETURNS TABLE(route_id uuid, timezone character varying, date_from date, date_to date)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT x.route_id, x.timezone, x.date_from, x.date_to
    FROM (
        SELECT
            r.id AS route_id,
            CAST(r.timezone AS VARCHAR) AS timezone,
            GREATEST(COALESCE(p_from, lt.today), lt.today) AS date_from,
            LEAST(COALESCE(p_to, lt.today + 14), lt.today + 14) AS date_to
        FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        CROSS JOIN LATERAL (SELECT CAST(now() AT TIME ZONE r.timezone AS DATE) AS today) lt
        WHERE tc.tenant_id = p_tenant_id
          AND (p_route_id IS NULL OR r.id = p_route_id)
    ) x
    -- A range wholly past the horizon (an exception months ahead) is nothing to do today; the daily
    -- pass reaches it when it comes into the window.
    WHERE x.date_from <= x.date_to;
$function$;

COMMENT ON FUNCTION tracking.fn_trip_window(uuid,uuid,date,date) IS 'The days the materialiser may touch per route: the route''s local today to today + 14, narrowed by an optional range (TRACK-008 D4)';
