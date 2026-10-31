CREATE OR REPLACE FUNCTION tracking.sp_rider_access_code_check(p_tenant_id uuid, p_code_hash bytea)
 RETURNS text
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_code tracking.rider_access_codes%ROWTYPE;
    v_name TEXT;
BEGIN
    SELECT c.* INTO v_code
    FROM tracking.rider_access_codes c
    WHERE c.tenant_id = p_tenant_id AND c.code_hash = p_code_hash;
    IF v_code.id IS NULL THEN
        RAISE EXCEPTION 'rider-access.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_code.used_at IS NOT NULL THEN
        RAISE EXCEPTION 'rider-access.used' USING ERRCODE = 'P0001';
    END IF;
    IF v_code.revoked_at IS NOT NULL OR v_code.expires_at <= now() THEN
        RAISE EXCEPTION 'rider-access.expired' USING ERRCODE = 'P0001';
    END IF;

    SELECT TRIM(r.first_name || ' ' || COALESCE(r.last_name, '')) INTO v_name
    FROM tracking.riders r
    WHERE r.id = v_code.rider_id;
    RETURN v_name;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_access_code_check(uuid,bytea) IS 'Whether an access code still opens, and the passenger''s name for the account it may create (MOBILE-042).';
