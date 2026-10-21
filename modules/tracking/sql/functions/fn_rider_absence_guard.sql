CREATE OR REPLACE FUNCTION tracking.fn_rider_absence_guard(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid, p_guardian_user_id uuid, p_write boolean DEFAULT false)
 RETURNS integer
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
    v_riders UUID[];
    v_guardian_scope UUID[];
    v_cutoff INT;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id, p_write);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    END IF;
    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    SELECT o.absence_cutoff_min INTO v_cutoff
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = p_rider_id
      AND o.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
      AND (v_riders IS NULL OR r.id = ANY(v_riders))
      AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope));

    IF v_cutoff IS NULL THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_cutoff;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_rider_absence_guard(uuid,uuid,uuid,uuid,boolean) IS 'Raises rider.not-found for a rider outside the tenant, the organization scope (its write scope with p_write, TRACK-067), the supervised riders or the guardian scope; answers the rider''s organization''s absence_cutoff_min (TRACK-022)';
