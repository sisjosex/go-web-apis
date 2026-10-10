CREATE OR REPLACE FUNCTION tracking.fn_driver_of_user(p_tenant_id uuid, p_user_id uuid)
 RETURNS tracking.drivers
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
BEGIN
    SELECT d.* INTO v_driver
    FROM tracking.drivers d
    WHERE d.tenant_id = p_tenant_id AND d.user_id = p_user_id;
    IF v_driver.id IS NULL THEN
        RAISE EXCEPTION 'driver.not-linked' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_driver;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_driver_of_user(uuid,uuid) IS 'The driver linked to a tenant account; raises driver.not-linked (TRACK-011)';
