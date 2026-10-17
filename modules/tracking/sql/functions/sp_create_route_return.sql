CREATE OR REPLACE FUNCTION tracking.sp_create_route_return(p_tenant_id uuid, p_route_id uuid, p_departure_time time without time zone, p_created_by uuid)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_out     tracking.routes%ROWTYPE;
    v_ret     UUID;
    v_today   DATE;
    v_version UUID;
    v_stops   JSONB;
    v_days    SMALLINT;
    v_item    JSONB;
BEGIN
    -- The outbound row is the lock: two calls for one route queue here, so only one return is made.
    SELECT r.* INTO v_out
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    FOR UPDATE OF r;
    IF v_out.id IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_out.direction <> 'outbound' THEN
        RAISE EXCEPTION 'route.not-outbound' USING ERRCODE = 'P0001';
    END IF;
    IF v_out.paired_route_id IS NOT NULL THEN
        RAISE EXCEPTION 'route.has-return' USING ERRCODE = 'P0001';
    END IF;
    IF p_departure_time IS NULL THEN
        RAISE EXCEPTION 'route.departure-required' USING ERRCODE = 'P0001';
    END IF;

    -- The stops of the version in force, reversed: the destination first (TRACK-038 D1).
    v_today := tracking.fn_route_today(v_out.id);
    SELECT rv.id INTO v_version
    FROM tracking.route_versions rv
    WHERE rv.route_id = v_out.id
      AND v_today >= rv.effective_from
      AND (rv.effective_to IS NULL OR v_today <= rv.effective_to);
    SELECT COALESCE(jsonb_agg(jsonb_build_object('stop_place_id', vs.stop_place_id) ORDER BY vs.sequence DESC), '[]'::jsonb)
      INTO v_stops
    FROM tracking.route_version_stops vs
    WHERE vs.version_id = v_version;

    SELECT r.id INTO v_ret FROM tracking.sp_create_route(
        p_tenant_id, v_out.company_id, TRIM(v_out.route_name) || ' (retorno)',
        v_out.destination_address, v_out.origin_address, 'inbound',
        v_out.vehicle_id, v_out.default_driver_id, v_out.timezone, NULL,
        v_out.destination_lat, v_out.destination_lng, v_out.origin_lat, v_out.origin_lng,
        v_out.estimated_duration_minutes, v_out.capacity) r;
    UPDATE tracking.routes r SET organization_id = v_out.organization_id, paired_route_id = v_out.id
    WHERE r.id = v_ret;
    UPDATE tracking.routes r SET paired_route_id = v_ret WHERE r.id = v_out.id;
    PERFORM tracking.sp_create_route_version(p_tenant_id, v_ret, tracking.fn_route_today(v_ret), v_stops, p_created_by);

    SELECT CAST(bit_or(s.days_of_week) AS SMALLINT) INTO v_days
    FROM tracking.route_schedules s
    WHERE s.route_id = v_out.id AND COALESCE(s.valid_until, v_today) >= v_today;
    PERFORM tracking.sp_create_route_schedule(p_tenant_id, v_ret, CAST(COALESCE(v_days, 31) AS SMALLINT), p_departure_time,
        tracking.fn_route_today(v_ret), NULL, NULL);

    -- The outbound's riders ride back, pickup and drop-off swapped, as the twin of a pair assignment
    -- (same days and start, so removing one removes the other); a rider who contracted only the
    -- outbound stays off it (TRACK-043 D1).
    FOR v_item IN
        SELECT jsonb_strip_nulls(jsonb_build_object(
                   'rider_id', a.rider_id, 'route_id', v_ret, 'days_of_week', a.days_of_week,
                   'pickup_stop_place_id', a.dropoff_stop_place_id,
                   'dropoff_stop_place_id', a.pickup_stop_place_id,
                   'valid_from', a.valid_from, 'valid_until', a.valid_until))
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.riders rd ON rd.id = a.rider_id
        WHERE a.route_id = v_out.id
          AND (a.valid_until IS NULL OR a.valid_until >= v_today)
          AND rd.service_legs <> 'outbound'
        ORDER BY a.valid_from, a.id
    LOOP
        PERFORM tracking.fn_assignment_put(p_tenant_id, NULL, v_item);
    END LOOP;

    RETURN v_ret;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_route_return(uuid,uuid,time without time zone,uuid) IS 'Gives an outbound route without a pair its return "<name> (retorno)": the stops in force reversed, same company, vehicle, driver and client, the outbound''s weekdays at p_departure_time, linked both ways, and the outbound''s riders (except those who contracted only the outbound) with pickup and drop-off swapped, in one transaction (TRACK-070 D2); raises tracking.route.not-found, tracking.route.not-outbound, tracking.route.has-return, tracking.route.departure-required';
