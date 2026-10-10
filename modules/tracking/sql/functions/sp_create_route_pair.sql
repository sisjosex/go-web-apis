CREATE OR REPLACE FUNCTION tracking.sp_create_route_pair(p_tenant_id uuid, p_company_id uuid, p_organization_id uuid, p_route_name character varying, p_vehicle_id uuid, p_with_return boolean, p_arrival_time time without time zone, p_departure_time time without time zone, p_days smallint, p_stops jsonb, p_timezone character varying, p_return_route_name character varying, p_destination_lat numeric, p_destination_lng numeric, p_destination_address character varying, p_destination_stop_place_id uuid, p_created_by uuid)
 RETURNS TABLE(route_id uuid, return_route_id uuid)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_org_name  VARCHAR;
    v_org_addr  VARCHAR;
    v_dest      GEOGRAPHY;
    v_dest_addr VARCHAR;
    v_dest_item JSONB;
    v_org_stop  UUID;
    v_stop_at   GEOGRAPHY;
    v_stops     JSONB := '[]'::jsonb;
    v_first     GEOGRAPHY;
    v_first_lbl VARCHAR;
    v_prev      GEOGRAPHY;
    v_point     GEOGRAPHY;
    v_metres    DOUBLE PRECISION := 0;
    v_count     INT := 0;
    v_minutes   INT;
    v_item      JSONB;
    v_out       UUID;
    v_ret       UUID;
    v_schedule  UUID;
    v_days      SMALLINT := COALESCE(p_days, 31);
BEGIN
    SELECT o.name, o.address, sp.id, sp.location
      INTO v_org_name, v_org_addr, v_org_stop, v_stop_at
    FROM tracking.organizations o
    LEFT JOIN tracking.stop_places sp ON sp.id = o.stop_place_id AND sp.location IS NOT NULL
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF v_org_name IS NULL THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_destination_stop_place_id IS NOT NULL THEN
        SELECT sp.location, COALESCE(NULLIF(TRIM(sp.address), ''), sp.name)
          INTO v_dest, v_dest_addr
        FROM tracking.stop_places sp
        WHERE sp.id = p_destination_stop_place_id AND sp.tenant_id = p_tenant_id AND sp.location IS NOT NULL;
        IF v_dest IS NULL THEN
            RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
        END IF;
        v_dest_item := jsonb_build_object('stop_place_id', p_destination_stop_place_id);
    ELSIF p_destination_lat IS NOT NULL AND p_destination_lng IS NOT NULL THEN
        v_dest := ST_SetSRID(ST_MakePoint(p_destination_lng, p_destination_lat), 4326)::geography;
        v_dest_addr := COALESCE(NULLIF(TRIM(p_destination_address), ''), v_org_addr, v_org_name);
        IF v_org_stop IS NOT NULL AND ST_DWithin(v_stop_at, v_dest, 30) THEN
            v_dest := v_stop_at;
            v_dest_item := jsonb_build_object('stop_place_id', v_org_stop);
        ELSE
            v_dest_item := jsonb_build_object(
                'name', v_org_name, 'address', v_dest_addr,
                'latitude', ST_Y(v_dest::geometry), 'longitude', ST_X(v_dest::geometry));
        END IF;
    ELSE
        RAISE EXCEPTION 'route.destination-required' USING ERRCODE = 'P0001';
    END IF;
    IF COALESCE(p_with_return, true) AND p_departure_time IS NULL THEN
        RAISE EXCEPTION 'route.departure-required' USING ERRCODE = 'P0001';
    END IF;

    -- The pickups in order, without the editor's own sequence (the list order is the order). A body
    -- without stops sends JSON null, which is not SQL NULL: anything but an array is no stops.
    FOR v_item IN SELECT e FROM jsonb_array_elements(
        CASE WHEN jsonb_typeof(p_stops) = 'array' THEN p_stops ELSE '[]'::jsonb END) e LOOP
        v_point := tracking.fn_stop_item_point(p_tenant_id, v_item);
        IF v_point IS NULL THEN
            RAISE EXCEPTION 'stop-place.invalid' USING ERRCODE = 'P0001';
        END IF;
        IF v_first IS NULL THEN
            v_first := v_point;
            v_first_lbl := COALESCE(NULLIF(TRIM(v_item->>'address'), ''), NULLIF(TRIM(v_item->>'name'), ''), (
                SELECT COALESCE(sp.address, sp.name) FROM tracking.stop_places sp
                WHERE sp.id = NULLIF(v_item->>'stop_place_id', '')::UUID AND sp.tenant_id = p_tenant_id));
        ELSE
            v_metres := v_metres + ST_Distance(v_prev, v_point);
        END IF;
        v_prev := v_point;
        v_count := v_count + 1;
        v_stops := v_stops || jsonb_build_array(v_item - 'sequence');
    END LOOP;
    IF v_prev IS NOT NULL THEN
        v_metres := v_metres + ST_Distance(v_prev, v_dest);
    END IF;
    v_minutes := GREATEST(5, CAST(CEIL((v_metres * 1.3 / 1000 / 25 * 60 + v_count) / 5.0) * 5 AS INT));

    SELECT r.id INTO v_out FROM tracking.sp_create_route(
        p_tenant_id, p_company_id, p_route_name,
        COALESCE(v_first_lbl, v_dest_addr), v_dest_addr, 'outbound',
        p_vehicle_id, NULL, p_timezone, NULL,
        CAST(ST_Y(v_first::geometry) AS DECIMAL), CAST(ST_X(v_first::geometry) AS DECIMAL),
        CAST(ST_Y(v_dest::geometry) AS DECIMAL), CAST(ST_X(v_dest::geometry) AS DECIMAL),
        v_minutes, NULL) r;
    UPDATE tracking.routes r SET organization_id = p_organization_id WHERE r.id = v_out;
    PERFORM tracking.sp_create_route_version(p_tenant_id, v_out, tracking.fn_route_today(v_out),
        v_stops || jsonb_build_array(v_dest_item), p_created_by);
    IF p_arrival_time IS NOT NULL THEN
        SELECT s.id INTO v_schedule FROM tracking.sp_create_route_schedule(p_tenant_id, v_out, v_days,
            CAST(p_arrival_time - make_interval(mins => v_minutes) AS TIME), tracking.fn_route_today(v_out), NULL, NULL) s;
        UPDATE tracking.route_schedules s SET departure_auto_min = v_minutes WHERE s.id = v_schedule;
    END IF;

    IF COALESCE(p_with_return, true) THEN
        SELECT r.id INTO v_ret FROM tracking.sp_create_route(
            p_tenant_id, p_company_id,
            COALESCE(NULLIF(TRIM(p_return_route_name), ''), TRIM(p_route_name) || ' (retorno)'),
            v_dest_addr, COALESCE(v_first_lbl, v_dest_addr), 'inbound',
            p_vehicle_id, NULL, p_timezone, NULL,
            CAST(ST_Y(v_dest::geometry) AS DECIMAL), CAST(ST_X(v_dest::geometry) AS DECIMAL),
            CAST(ST_Y(v_first::geometry) AS DECIMAL), CAST(ST_X(v_first::geometry) AS DECIMAL),
            v_minutes, NULL) r;
        UPDATE tracking.routes r SET organization_id = p_organization_id, paired_route_id = v_out
        WHERE r.id = v_ret;
        UPDATE tracking.routes r SET paired_route_id = v_ret WHERE r.id = v_out;
        PERFORM tracking.sp_create_route_version(p_tenant_id, v_ret, tracking.fn_route_today(v_ret),
            jsonb_build_array(v_dest_item) || COALESCE((
                SELECT jsonb_agg(e.value ORDER BY e.ordinality DESC)
                FROM jsonb_array_elements(v_stops) WITH ORDINALITY AS e(value, ordinality)
            ), '[]'::jsonb),
            p_created_by);
        PERFORM tracking.sp_create_route_schedule(p_tenant_id, v_ret, v_days, p_departure_time,
            tracking.fn_route_today(v_ret), NULL, NULL);
    END IF;

    RETURN QUERY SELECT v_out, v_ret;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_route_pair(uuid,uuid,uuid,character varying,uuid,boolean,time without time zone,time without time zone,smallint,jsonb,character varying,character varying,numeric,numeric,character varying,uuid,uuid) IS 'Creates a destination''s outbound route (stops, possibly none, then the destination) and, with p_with_return, its return "<name> (retorno)" (reversed), same vehicle, linked both ways, each with its schedule, in one transaction (TRACK-038 D1, D2); the outbound departure follows its stops (TRACK-044 D2); the destination is the stop sent, or a pin (TRACK-049 D3); raises tracking.organization.not-found, tracking.stop-place.not-found, tracking.route.destination-required, tracking.route.departure-required';
