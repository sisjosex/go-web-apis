CREATE OR REPLACE FUNCTION tracking.sp_rider_qr_token(p_tenant_id uuid, p_rider_id uuid, p_rotate boolean, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS text
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_token TEXT;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF p_rotate THEN
        UPDATE tracking.riders r
        SET qr_token = tracking.fn_new_qr_token(), updated_at = CURRENT_TIMESTAMP
        FROM tracking.organizations o
        WHERE o.id = r.organization_id
          AND r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR o.id = ANY(v_scope))
        RETURNING r.qr_token INTO v_token;
    ELSE
        SELECT r.qr_token INTO v_token
        FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR o.id = ANY(v_scope));
    END IF;

    IF v_token IS NULL THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_token;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_qr_token(uuid,uuid,boolean,uuid) IS 'A rider''s card token, replaced first when p_rotate (the old one then resolves nothing); raises rider.not-found, also out of an organization user''s scope (TRACK-027 D1)';
