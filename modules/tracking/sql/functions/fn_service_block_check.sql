CREATE OR REPLACE FUNCTION tracking.fn_service_block_check(p_tenant_id uuid, p_vehicle_id uuid, p_driver_id uuid, p_driver_field character varying DEFAULT 'driver_id'::character varying)
 RETURNS void
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    IF p_vehicle_id IS NOT NULL AND tracking.fn_service_blocked(p_tenant_id, 'vehicle', p_vehicle_id) THEN
        RAISE EXCEPTION 'service.blocked' USING ERRCODE = 'P0001', DETAIL = 'vehicle_id';
    END IF;

    IF p_driver_id IS NOT NULL AND tracking.fn_service_blocked(p_tenant_id, 'driver', p_driver_id) THEN
        RAISE EXCEPTION 'service.blocked' USING ERRCODE = 'P0001', DETAIL = p_driver_field;
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_service_block_check(uuid,uuid,uuid,character varying) IS 'Raises service.blocked, DETAIL naming the field (vehicle_id, or p_driver_field for the driver), when the vehicle or driver being assigned carries an expired document of a blocks_service type; NULL ids are not checked (TRACK-064 D1)';
