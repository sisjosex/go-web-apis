CREATE OR REPLACE FUNCTION tracking.sp_driver_account_set(p_tenant_id uuid, p_driver_id uuid, p_user_id uuid)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_previous UUID;
BEGIN
    SELECT d.user_id INTO v_previous
    FROM tracking.drivers d
    WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_user_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id AND d.user_id = p_user_id AND d.id <> p_driver_id
    ) THEN
        RAISE EXCEPTION 'driver.user-already-linked' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.drivers d SET user_id = p_user_id, updated_at = CURRENT_TIMESTAMP
    WHERE d.id = p_driver_id;

    RETURN NULLIF(v_previous, p_user_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_driver_account_set(uuid,uuid,uuid) IS 'Links (or with NULL clears) a driver''s app account and answers the account it replaced; raises tracking.driver.not-found or tracking.driver.user-already-linked';
