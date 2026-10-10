CREATE OR REPLACE FUNCTION tracking.sp_delete_vehicle(p_vehicle_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles WHERE tracking.vehicles.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Delete the vehicle (CASCADE will handle related records)
    DELETE FROM tracking.vehicles WHERE tracking.vehicles.id = p_vehicle_id;

    RETURN TRUE;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_delete_vehicle(p_tenant_id uuid, p_vehicle_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.vehicles v WHERE v.id = p_vehicle_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$function$;
