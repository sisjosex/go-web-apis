CREATE OR REPLACE FUNCTION tracking.sp_rider_access_code_revoke(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id, true);

    UPDATE tracking.rider_access_codes
    SET revoked_at = now()
    WHERE rider_id = p_rider_id AND used_at IS NULL AND revoked_at IS NULL;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_access_code_revoke(uuid,uuid,uuid) IS 'Ends a passenger''s live access code, if any (MOBILE-042).';
