CREATE OR REPLACE FUNCTION tracking.sp_apply_optimization_run(p_tenant_id uuid, p_run_id uuid, p_routes jsonb, p_effective_from date, p_user_id uuid)
 RETURNS TABLE(routes_created integer, assignments_created integer)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION tracking.sp_apply_optimization_run(uuid,uuid,jsonb,date,uuid) IS 'Applies a done run as the operator adjusted it: one route pair per vehicle with its riders assigned from p_effective_from, all or nothing; raises optimization.not-found, optimization.applied, optimization.not-ready, optimization.vehicle-repeated, vehicle.not-found and optimization.stale with the rider that can no longer be assigned as DETAIL (TRACK-014 step 3)';
