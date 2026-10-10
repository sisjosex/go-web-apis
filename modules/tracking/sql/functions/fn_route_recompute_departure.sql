CREATE OR REPLACE FUNCTION tracking.fn_route_recompute_departure(p_tenant_id uuid, p_route_id uuid, p_on date)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_minutes INT;
BEGIN
    SELECT tracking.fn_route_version_estimate(p_tenant_id, rv.id) INTO v_minutes
    FROM tracking.route_versions rv
    WHERE rv.route_id = p_route_id
      AND p_on >= rv.effective_from
      AND (rv.effective_to IS NULL OR p_on <= rv.effective_to);
    IF v_minutes IS NULL THEN
        RETURN;
    END IF;

    UPDATE tracking.route_schedules s
       SET start_time = s.start_time + make_interval(mins => s.departure_auto_min - v_minutes),
           departure_auto_min = v_minutes,
           updated_at = CURRENT_TIMESTAMP
     WHERE s.route_id = p_route_id
       AND s.departure_auto_min IS NOT NULL
       AND s.departure_auto_min <> v_minutes
       AND (s.valid_until IS NULL OR s.valid_until >= p_on);
    IF FOUND THEN
        -- Until the line is traced and writes the real one, the trips end by the estimate.
        UPDATE tracking.routes r SET estimated_duration_minutes = v_minutes, updated_at = CURRENT_TIMESTAMP
        WHERE r.id = p_route_id;
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_recompute_departure(uuid,uuid,date) IS 'Moves the route''s auto departures (departure_auto_min) to arrive when they did with the stops in force on p_on, by the straight-line estimate (TRACK-044 D2)';
