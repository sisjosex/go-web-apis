CREATE OR REPLACE FUNCTION tracking.fn_route_changed(p_route_id uuid, p_date_from date, p_date_to date)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    INSERT INTO tracking.outbox (topic, payload)
    VALUES ('route.changed', jsonb_build_object(
        'route_id',  p_route_id,
        'date_from', p_date_from,
        'date_to',   p_date_to
    ));
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_changed(uuid,date,date) IS 'Writes one route.changed outbox row for a route over a span of days; date_to NULL is "and every day after" (TRACK-018 step 4)';
