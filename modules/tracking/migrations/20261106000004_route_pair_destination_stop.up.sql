-- TRACK-049 D3: the route form names the destination stop. sp_create_route_pair takes
-- p_destination_stop_place_id (a stop of the tenant) or the pin (p_destination_lat/lng, a new stop
-- named after the client, or the client's own stop when it lies within 30 m); with neither it raises
-- route.destination-required. A client pin is no longer turned into a stop on its own.
-- D1: the return route defaults to "<name> (retorno)"; existing routes keep their name.

DROP FUNCTION IF EXISTS tracking.sp_create_route_pair(UUID, UUID, UUID, VARCHAR, UUID, BOOLEAN, TIME, TIME, SMALLINT, JSONB, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, UUID);

CREATE FUNCTION tracking.sp_create_route_pair(
    p_tenant_id                 UUID,
    p_company_id                UUID,
    p_organization_id           UUID,
    p_route_name                VARCHAR(255),
    p_vehicle_id                UUID,
    p_with_return               BOOLEAN,
    p_arrival_time              TIME,
    p_departure_time            TIME,
    p_days                      SMALLINT,
    p_stops                     JSONB,
    p_timezone                  VARCHAR(64),
    p_return_route_name         VARCHAR(255),
    p_destination_lat           DECIMAL,
    p_destination_lng           DECIMAL,
    p_destination_address       VARCHAR(500),
    p_destination_stop_place_id UUID,
    p_created_by                UUID
)
RETURNS TABLE(route_id UUID, return_route_id UUID) AS $$
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
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_route_pair(UUID, UUID, UUID, VARCHAR, UUID, BOOLEAN, TIME, TIME, SMALLINT, JSONB, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, UUID, UUID) IS
'Creates a destination''s outbound route (stops, possibly none, then the destination) and, with p_with_return, its return "<name> (retorno)" (reversed), same vehicle, linked both ways, each with its schedule, in one transaction (TRACK-038 D1, D2); the outbound departure follows its stops (TRACK-044 D2); the destination is the stop sent, or a pin (TRACK-049 D3); raises tracking.organization.not-found, tracking.stop-place.not-found, tracking.route.destination-required, tracking.route.departure-required';

-- Planning's apply names the destination the way the form does: the client's stop, else its pin.
CREATE OR REPLACE FUNCTION tracking.sp_apply_optimization_run(
    p_tenant_id      UUID,
    p_run_id         UUID,
    p_routes         JSONB,
    p_effective_from DATE,
    p_user_id        UUID
)
RETURNS TABLE(routes_created INT, assignments_created INT) AS $$
DECLARE
    v_run       RECORD;
    v_org       RECORD;
    v_route     JSONB;
    v_vehicle   RECORD;
    v_riders    UUID[];
    v_stops     JSONB;
    v_out       UUID;
    v_skipped   RECORD;
    v_routes    INT := 0;
    v_assigned  INT := 0;
    v_departure TIME;
    v_days      SMALLINT;
BEGIN
    SELECT r.* INTO v_run
    FROM tracking.optimization_runs r
    WHERE r.id = p_run_id AND r.tenant_id = p_tenant_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'optimization.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_run.applied_at IS NOT NULL THEN
        RAISE EXCEPTION 'optimization.applied' USING ERRCODE = 'P0001';
    END IF;
    IF v_run.status <> 'done' THEN
        RAISE EXCEPTION 'optimization.not-ready' USING ERRCODE = 'P0001';
    END IF;
    IF (SELECT COUNT(*) FROM jsonb_array_elements(p_routes) e) <> (SELECT COUNT(DISTINCT e->>'vehicle_id') FROM jsonb_array_elements(p_routes) e) THEN
        RAISE EXCEPTION 'optimization.vehicle-repeated' USING ERRCODE = 'P0001';
    END IF;

    SELECT o.id, o.name, o.timezone, o.stop_place_id,
           CAST(ST_Y(o.location::geometry) AS DECIMAL) AS lat, CAST(ST_X(o.location::geometry) AS DECIMAL) AS lng
      INTO v_org
    FROM tracking.organizations o WHERE o.id = v_run.organization_id AND o.tenant_id = p_tenant_id;
    v_departure := CAST(NULLIF(v_run.params->>'departure_time', '') AS TIME);
    v_days := CAST(v_run.params->>'days' AS SMALLINT);

    FOR v_route IN SELECT e FROM jsonb_array_elements(COALESCE(p_routes, '[]'::jsonb)) e LOOP
        SELECT ARRAY(SELECT CAST(x AS UUID) FROM jsonb_array_elements_text(COALESCE(v_route->'rider_ids', '[]'::jsonb)) x)
          INTO v_riders;
        CONTINUE WHEN cardinality(v_riders) = 0;

        SELECT v.id, v.company_id, v.plate_number INTO v_vehicle
        FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
        WHERE v.id = CAST(v_route->>'vehicle_id' AS UUID);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
        END IF;

        -- The pickups in the given order: the rider's shared stop, else a point at home named after them.
        SELECT jsonb_agg(CASE WHEN rd.stop_place_id IS NOT NULL
                THEN jsonb_build_object('stop_place_id', rd.stop_place_id)
                ELSE jsonb_build_object('name', TRIM(rd.first_name || ' ' || rd.last_name),
                    'latitude', ST_Y(rd.home_location::geometry), 'longitude', ST_X(rd.home_location::geometry)) END
                ORDER BY u.ord)
          INTO v_stops
        FROM unnest(v_riders) WITH ORDINALITY AS u(id, ord)
        INNER JOIN tracking.riders rd ON rd.id = u.id AND rd.organization_id = v_run.organization_id
        WHERE rd.stop_place_id IS NOT NULL OR rd.home_location IS NOT NULL;
        IF COALESCE(jsonb_array_length(v_stops), 0) <> cardinality(v_riders) THEN
            RAISE EXCEPTION 'optimization.stale' USING ERRCODE = 'P0001',
                DETAIL = (SELECT CAST(u.id AS TEXT) FROM unnest(v_riders) u(id)
                          LEFT JOIN tracking.riders rd ON rd.id = u.id AND rd.organization_id = v_run.organization_id
                          WHERE rd.id IS NULL OR (rd.stop_place_id IS NULL AND rd.home_location IS NULL) LIMIT 1);
        END IF;

        SELECT p.route_id INTO v_out FROM tracking.sp_create_route_pair(
            p_tenant_id, v_vehicle.company_id, v_org.id,
            CAST(v_org.name || ' · ' || v_vehicle.plate_number AS VARCHAR), v_vehicle.id,
            v_departure IS NOT NULL, CAST(v_run.params->>'arrival_time' AS TIME), v_departure, v_days,
            v_stops, v_org.timezone, NULL, v_org.lat, v_org.lng, NULL, v_org.stop_place_id, p_user_id) p;
        v_routes := v_routes + 1;

        SELECT b.rider_id, b.reason INTO v_skipped
        FROM tracking.sp_bulk_assign_riders(p_tenant_id, v_out, v_riders, v_days, p_effective_from) b
        WHERE b.status <> 'assigned'
        LIMIT 1;
        IF FOUND THEN
            RAISE EXCEPTION 'optimization.stale' USING ERRCODE = 'P0001', DETAIL = CAST(v_skipped.rider_id AS TEXT);
        END IF;
        v_assigned := v_assigned + cardinality(v_riders);
    END LOOP;

    UPDATE tracking.optimization_runs r
       SET applied_at = now(), applied_by = p_user_id, updated_at = now()
     WHERE r.id = p_run_id;

    RETURN QUERY SELECT v_routes, v_assigned;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_apply_optimization_run(UUID, UUID, JSONB, DATE, UUID) IS
'Applies a done run as the operator adjusted it: one route pair per vehicle with its riders assigned from p_effective_from, all or nothing; raises optimization.not-found, optimization.applied, optimization.not-ready, optimization.vehicle-repeated, vehicle.not-found and optimization.stale with the rider that can no longer be assigned as DETAIL (TRACK-014 step 3)';
