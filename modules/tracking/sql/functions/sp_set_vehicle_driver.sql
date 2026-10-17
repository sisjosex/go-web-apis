CREATE OR REPLACE FUNCTION tracking.sp_set_vehicle_driver(p_tenant_id uuid, p_vehicle_id uuid, p_driver_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_company_id UUID;
BEGIN
    SELECT v.company_id INTO v_company_id
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
    WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id;

    IF v_company_id IS NULL THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_driver_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = p_driver_id AND d.company_id = v_company_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.driver-company-mismatch' USING ERRCODE = 'P0001';
    END IF;
    PERFORM tracking.fn_service_block_check(p_tenant_id, NULL, p_driver_id);

    UPDATE tracking.vehicles v
    SET default_driver_id = p_driver_id, updated_at = CURRENT_TIMESTAMP
    WHERE v.id = p_vehicle_id;

    -- Trips already materialised for its routes take the new driver from today on, unless the route
    -- or the day names its own.
    INSERT INTO tracking.outbox (topic, payload)
    SELECT 'route.changed', jsonb_build_object('route_id', r.id, 'date_from', CURRENT_DATE, 'date_to', NULL)
    FROM tracking.routes r
    WHERE r.vehicle_id = p_vehicle_id AND r.default_driver_id IS NULL;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_set_vehicle_driver(uuid,uuid,uuid) IS 'Sets (NULL clears) the usual driver of a vehicle, who must belong to its carrier, and replans its routes from today; raises vehicle.not-found, vehicle.driver-company-mismatch (TRACK-034 D1) or service.blocked (TRACK-064)';
