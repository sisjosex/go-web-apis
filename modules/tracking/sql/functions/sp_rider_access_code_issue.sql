CREATE OR REPLACE FUNCTION tracking.sp_rider_access_code_issue(p_tenant_id uuid, p_rider_id uuid, p_code_hash bytea, p_created_by uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS timestamp with time zone
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_expires TIMESTAMPTZ := now() + INTERVAL '7 days';
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id, true);

    UPDATE tracking.rider_access_codes
    SET revoked_at = now()
    WHERE rider_id = p_rider_id AND used_at IS NULL AND revoked_at IS NULL;

    INSERT INTO tracking.rider_access_codes (tenant_id, rider_id, code_hash, expires_at, created_by)
    VALUES (p_tenant_id, p_rider_id, p_code_hash, v_expires, p_created_by);

    RETURN v_expires;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_access_code_issue(uuid,uuid,bytea,uuid,uuid) IS 'A new one-time access code for a passenger, seven days; the live one is revoked (MOBILE-042 D1).';
