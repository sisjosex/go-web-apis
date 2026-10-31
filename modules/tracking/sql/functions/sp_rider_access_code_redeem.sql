CREATE OR REPLACE FUNCTION tracking.sp_rider_access_code_redeem(p_tenant_id uuid, p_code_hash bytea, p_user_id uuid)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_code tracking.rider_access_codes%ROWTYPE;
    v_name TEXT;
BEGIN
    -- The row is locked: two phones with one code, the second is told it was used.
    SELECT c.* INTO v_code
    FROM tracking.rider_access_codes c
    WHERE c.tenant_id = p_tenant_id AND c.code_hash = p_code_hash
    FOR UPDATE;
    IF v_code.id IS NULL THEN
        RAISE EXCEPTION 'rider-access.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_code.used_at IS NOT NULL THEN
        RAISE EXCEPTION 'rider-access.used' USING ERRCODE = 'P0001';
    END IF;
    IF v_code.revoked_at IS NOT NULL OR v_code.expires_at <= now() THEN
        RAISE EXCEPTION 'rider-access.expired' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.rider_access_codes
    SET used_at = now(), used_by = p_user_id
    WHERE id = v_code.id;

    -- The account becomes the rider's own contact: that is what gives it the pass (MOBILE-039 D1).
    UPDATE tracking.rider_contacts
    SET user_id = p_user_id, updated_at = CURRENT_TIMESTAMP
    WHERE rider_id = v_code.rider_id AND relation = 'self';
    IF NOT FOUND THEN
        SELECT TRIM(r.first_name || ' ' || COALESCE(r.last_name, '')) INTO v_name
        FROM tracking.riders r
        WHERE r.id = v_code.rider_id;
        INSERT INTO tracking.rider_contacts (rider_id, relation, name, user_id)
        VALUES (v_code.rider_id, 'self', v_name, p_user_id);
    END IF;

    RETURN v_code.rider_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_access_code_redeem(uuid,bytea,uuid) IS 'Uses an access code once: the signed-in account becomes the passenger''s own contact (MOBILE-042).';
