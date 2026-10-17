CREATE OR REPLACE FUNCTION tracking.sp_update_trip(p_tenant_id uuid, p_trip_id uuid, p_changes jsonb, p_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, service_block jsonb, stops jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_trip    tracking.trips%ROWTYPE;
    v_company UUID;
    v_vehicle UUID;
    v_driver  UUID;
    v_start   TIMESTAMPTZ;
BEGIN
    v_trip := tracking.fn_trip_lock(p_tenant_id, p_trip_id);
    IF v_trip.status NOT IN ('planned', 'in_progress') THEN
        RAISE EXCEPTION 'trip.invalid-transition' USING ERRCODE = 'P0001';
    END IF;

    v_vehicle := CASE WHEN p_changes ? 'vehicle_id' THEN (p_changes->>'vehicle_id')::UUID ELSE v_trip.vehicle_id END;
    v_driver  := CASE WHEN p_changes ? 'driver_id' THEN (p_changes->>'driver_id')::UUID ELSE v_trip.driver_id END;
    v_start   := CASE WHEN p_changes ? 'planned_start' THEN (p_changes->>'planned_start')::TIMESTAMPTZ ELSE v_trip.planned_start END;

    SELECT r.company_id INTO v_company FROM tracking.routes r WHERE r.id = v_trip.route_id;
    IF v_vehicle IS DISTINCT FROM v_trip.vehicle_id AND v_vehicle IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v WHERE v.id = v_vehicle AND v.company_id = v_company
    ) THEN
        RAISE EXCEPTION 'route.company-mismatch' USING ERRCODE = 'P0001', DETAIL = 'vehicle_id';
    END IF;
    IF v_driver IS DISTINCT FROM v_trip.driver_id AND v_driver IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = v_driver AND d.company_id = v_company AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.company-mismatch' USING ERRCODE = 'P0001', DETAIL = 'driver_id';
    END IF;
    PERFORM tracking.fn_service_block_check(p_tenant_id,
        CASE WHEN v_vehicle IS DISTINCT FROM v_trip.vehicle_id THEN v_vehicle END,
        CASE WHEN v_driver IS DISTINCT FROM v_trip.driver_id THEN v_driver END);

    UPDATE tracking.trips t
    SET vehicle_id = v_vehicle,
        driver_id = v_driver,
        planned_start = v_start,
        is_overridden = TRUE,
        updated_at = now()
    WHERE t.id = p_trip_id;

    IF v_start <> v_trip.planned_start THEN
        UPDATE tracking.trip_stops ts
        SET planned_at = ts.planned_at + (v_start - v_trip.planned_start), updated_at = now()
        WHERE ts.trip_id = p_trip_id;
    END IF;

    PERFORM tracking.fn_trip_changed(p_tenant_id, p_trip_id, 'override', p_user_id, p_changes);

    RETURN QUERY SELECT * FROM tracking.sp_get_trip(p_tenant_id, p_trip_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_trip(uuid,uuid,jsonb,uuid) IS 'Overrides a trip not ended — vehicle_id, driver_id, planned_start, a key left out keeps its value — and marks it is_overridden so the materialiser keeps the header; raises trip.invalid-transition, route.company-mismatch, service.blocked for a newly assigned vehicle or driver (TRACK-064) (DETAIL the field); answers the detail (TRACK-020)';
