CREATE OR REPLACE FUNCTION tracking.sp_create_optimization_run(p_tenant_id uuid, p_organization_id uuid, p_params jsonb, p_user_id uuid)
 RETURNS TABLE(run_id uuid, cached boolean)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_days      SMALLINT := COALESCE(CAST(p_params->>'days' AS SMALLINT), 31);
    v_arrival   TIME := CAST(p_params->>'arrival_time' AS TIME);
    v_departure TIME := CAST(NULLIF(p_params->>'departure_time', '') AS TIME);
    v_max_ride  INT := COALESCE(CAST(p_params->>'max_ride_min' AS INT), 60);
    v_max       INT := LEAST(COALESCE(CAST(p_params->>'max_riders' AS INT), 100), 100);
    v_total     INT;
    v_dest      GEOGRAPHY;
    v_riders    JSONB;
    v_vehicles  JSONB;
    v_input     JSONB;
    v_hash      TEXT;
    v_run       UUID;
BEGIN
    SELECT o.location INTO v_dest
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_dest IS NULL THEN
        RAISE EXCEPTION 'route.destination-required' USING ERRCODE = 'P0001';
    END IF;

    WITH candidates AS (
        SELECT p.rider_id, p.lat, p.lng,
               atan2(CAST(p.lat AS DOUBLE PRECISION) - ST_Y(v_dest::geometry),
                     CAST(p.lng AS DOUBLE PRECISION) - ST_X(v_dest::geometry)) AS bearing
        FROM tracking.sp_planning_overview(p_tenant_id, p_organization_id, v_days) p
        WHERE p.lat IS NOT NULL
          AND ((p.service_legs = 'both' AND p.outbound_route_id IS NULL AND p.return_route_id IS NULL)
            OR (p.service_legs = 'outbound' AND p.outbound_route_id IS NULL)
            OR (p.service_legs = 'return' AND v_departure IS NOT NULL AND p.return_route_id IS NULL))
    ),
    batch AS (
        SELECT c.* FROM candidates c ORDER BY c.bearing, c.rider_id LIMIT v_max
    )
    SELECT (SELECT COUNT(*) FROM candidates),
           (SELECT jsonb_agg(jsonb_build_object('id', b.rider_id, 'lat', b.lat, 'lng', b.lng) ORDER BY b.rider_id) FROM batch b)
      INTO v_total, v_riders;
    IF v_riders IS NULL THEN
        RAISE EXCEPTION 'optimization.no-riders' USING ERRCODE = 'P0001';
    END IF;

    SELECT jsonb_agg(jsonb_build_object('id', v.id, 'seats', v.capacity) ORDER BY v.id)
      INTO v_vehicles
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    WHERE v.id IN (SELECT CAST(e AS UUID) FROM jsonb_array_elements_text(COALESCE(p_params->'vehicle_ids', '[]'::jsonb)) e)
      AND COALESCE(v.status, 'active') = 'active'
      AND COALESCE(v.capacity, 0) > 0;
    IF v_vehicles IS NULL THEN
        RAISE EXCEPTION 'optimization.no-vehicles' USING ERRCODE = 'P0001';
    END IF;

    v_input := jsonb_build_object(
        'destination', jsonb_build_object('lat', ST_Y(v_dest::geometry), 'lng', ST_X(v_dest::geometry)),
        'arrival_time', TO_CHAR(v_arrival, 'HH24:MI'),
        'max_ride_min', v_max_ride,
        'riders', v_riders,
        'deferred', v_total - jsonb_array_length(v_riders),
        'vehicles', v_vehicles);
    v_hash := md5(v_input::text || '|' || v_days || '|' || COALESCE(TO_CHAR(v_departure, 'HH24:MI'), ''));

    SELECT r.id INTO v_run
    FROM tracking.optimization_runs r
    WHERE r.tenant_id = p_tenant_id
      AND r.organization_id = p_organization_id
      AND r.input_hash = v_hash
      AND r.status <> 'failed'
      AND r.applied_at IS NULL
      AND r.created_at > now() - INTERVAL '24 hours'
    ORDER BY r.created_at DESC
    LIMIT 1;
    IF v_run IS NOT NULL THEN
        RETURN QUERY SELECT v_run, true;
        RETURN;
    END IF;

    INSERT INTO tracking.optimization_runs (tenant_id, organization_id, input_hash, params, input, created_by)
    VALUES (p_tenant_id, p_organization_id, v_hash,
            jsonb_build_object('days', v_days, 'arrival_time', TO_CHAR(v_arrival, 'HH24:MI'),
                'departure_time', TO_CHAR(v_departure, 'HH24:MI'), 'max_ride_min', v_max_ride,
                'vehicle_ids', COALESCE(p_params->'vehicle_ids', '[]'::jsonb)),
            v_input, p_user_id)
    RETURNING tracking.optimization_runs.id INTO v_run;

    INSERT INTO tracking.outbox (topic, payload, tenant_id)
    VALUES ('optimization.requested', jsonb_build_object('run_id', v_run, 'tenant_id', p_tenant_id), p_tenant_id);

    RETURN QUERY SELECT v_run, false;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_optimization_run(uuid,uuid,jsonb,uuid) IS 'Queues a route proposal for the organization''s riders still without a route and the chosen vehicles, publishing optimization.requested in the same transaction, or answers the same problem''s run of the last 24 h (cached); raises organization.not-found, route.destination-required, optimization.no-riders, optimization.no-vehicles (TRACK-014 step 2)';
