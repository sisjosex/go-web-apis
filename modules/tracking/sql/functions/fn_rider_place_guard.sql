CREATE OR REPLACE FUNCTION tracking.fn_rider_place_guard(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid, p_write boolean DEFAULT false)
 RETURNS void
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
    v_riders UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id, p_write);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
          AND (v_riders IS NULL OR r.id = ANY(v_riders))
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_rider_place_guard(uuid,uuid,uuid,boolean) IS 'Raises rider.not-found for a rider outside the tenant, p_scope_user_id''s organizations (those it administers with p_write) or its supervised riders (TRACK-044 D4, TRACK-067)';
