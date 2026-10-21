CREATE OR REPLACE FUNCTION tracking.sp_rider_guardian_remove(p_tenant_id uuid, p_rider_id uuid, p_user_id uuid, p_scope_user_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count INT;
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id, true);

    DELETE FROM tracking.rider_contacts rc
    WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian' AND rc.user_id = p_user_id
      AND NOT rc.is_primary;
    GET DIAGNOSTICS v_count = ROW_COUNT;

    UPDATE tracking.rider_contacts rc SET user_id = NULL, updated_at = CURRENT_TIMESTAMP
    WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian' AND rc.user_id = p_user_id
      AND rc.is_primary;

    IF v_count = 0 AND NOT FOUND THEN
        RAISE EXCEPTION 'rider.guardian-not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_guardian_remove(uuid,uuid,uuid,uuid) IS 'Unlinks a guardian account from a rider; raises tracking.rider.guardian-not-found when it was not linked';
