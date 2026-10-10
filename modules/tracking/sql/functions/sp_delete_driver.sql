CREATE OR REPLACE FUNCTION tracking.sp_delete_driver(p_tenant_id uuid, p_driver_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.routes r WHERE r.default_driver_id = p_driver_id
    ) THEN
        RAISE EXCEPTION 'driver.has-routes' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.drivers d
    WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id;
    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    RETURN v_deleted_count > 0;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_driver(uuid,uuid) IS 'Deletes a driver; raises tracking.driver.has-routes while a route still names them as its default driver';
