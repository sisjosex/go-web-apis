CREATE OR REPLACE FUNCTION tracking.sp_move_vehicle_company(p_tenant_id uuid, p_vehicle_id uuid, p_company_id uuid, p_move_routes boolean)
 RETURNS TABLE(routes_moved integer, routes_unassigned integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_vehicle tracking.vehicles%ROWTYPE;
    v_routes  UUID[];
    v_touched UUID[];
    v_moved   INT := 0;
    v_cleared INT := 0;
    v_route   UUID;
BEGIN
    -- The vehicle row is the lock: two moves of one vehicle queue here.
    SELECT v.* INTO v_vehicle
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
    WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    FOR UPDATE OF v;
    IF v_vehicle.id IS NULL THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF p_company_id = v_vehicle.company_id THEN
        RETURN QUERY SELECT 0, 0;
        RETURN;
    END IF;
    IF EXISTS (
        SELECT 1 FROM tracking.vehicles v WHERE v.company_id = p_company_id AND v.plate_number = v_vehicle.plate_number
    ) THEN
        RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
    END IF;

    v_routes := ARRAY(SELECT r.id FROM tracking.routes r WHERE r.vehicle_id = p_vehicle_id);

    IF COALESCE(p_move_routes, true) THEN
        -- TRACK-070 D4: the routes go with the vehicle, each with its pair; a pair running another
        -- bus loses it (that bus stays behind), and a usual driver of the old carrier is dropped.
        v_touched := ARRAY(
            SELECT DISTINCT u.id FROM tracking.routes r
            CROSS JOIN LATERAL (VALUES (r.id), (r.paired_route_id)) u(id)
            WHERE r.id = ANY(v_routes) AND u.id IS NOT NULL);
        IF EXISTS (
            SELECT 1 FROM tracking.routes r
            INNER JOIN tracking.routes t ON t.company_id = p_company_id AND t.route_code = r.route_code
            WHERE r.id = ANY(v_touched)
        ) THEN
            RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
        END IF;

        SELECT CAST(COUNT(*) AS INT) INTO v_cleared
        FROM tracking.routes r
        WHERE r.id = ANY(v_touched) AND r.vehicle_id IS NOT NULL AND r.vehicle_id <> p_vehicle_id;
        UPDATE tracking.routes r
        SET company_id = p_company_id,
            vehicle_id = CASE WHEN r.vehicle_id = p_vehicle_id THEN r.vehicle_id END,
            default_driver_id = CASE WHEN EXISTS (
                SELECT 1 FROM tracking.drivers d WHERE d.id = r.default_driver_id AND d.company_id = p_company_id
            ) THEN r.default_driver_id END,
            updated_at = CURRENT_TIMESTAMP
        WHERE r.id = ANY(v_touched);
        v_moved := cardinality(v_touched);
    ELSE
        -- The routes stay with their carrier and lose the bus: their trips go on planned, without one.
        v_touched := v_routes;
        UPDATE tracking.routes r SET vehicle_id = NULL, updated_at = CURRENT_TIMESTAMP
        WHERE r.id = ANY(v_touched);
        v_cleared := cardinality(v_touched);
    END IF;

    UPDATE tracking.vehicles v
    SET company_id = p_company_id,
        default_driver_id = CASE WHEN EXISTS (
            SELECT 1 FROM tracking.drivers d WHERE d.id = v.default_driver_id AND d.company_id = p_company_id
        ) THEN v.default_driver_id END,
        updated_at = CURRENT_TIMESTAMP
    WHERE v.id = p_vehicle_id;

    -- Planned trips follow from each route's today; the ones already driven keep their vehicle.
    FOREACH v_route IN ARRAY v_touched LOOP
        PERFORM tracking.fn_route_changed(v_route, tracking.fn_route_today(v_route), NULL);
    END LOOP;

    RETURN QUERY SELECT v_moved, v_cleared;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_move_vehicle_company(uuid,uuid,uuid,boolean) IS 'Moves a vehicle to another of the tenant''s carriers; with p_move_routes its routes and their pairs go with it (a pair on another bus loses it), without it its routes stay and lose the vehicle; a usual driver of another carrier is dropped and planned trips are replanned from today (TRACK-070 D4); answers how many routes moved and how many were left without a vehicle; raises tracking.vehicle.not-found, tracking.company.not-found, tracking.vehicle.plate-already-exists, tracking.route.code-already-exists';
