CREATE OR REPLACE FUNCTION tracking.sp_delete_rider(p_rider_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_rows_affected INT;
BEGIN
    -- Check if rider exists
    IF NOT EXISTS (SELECT 1 FROM tracking.riders WHERE tracking.riders.id = p_rider_id) THEN
        RAISE EXCEPTION 'tracking.rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Soft delete: set status to inactive
    UPDATE tracking.riders
    SET status = 'inactive', updated_at = CURRENT_TIMESTAMP
    WHERE tracking.riders.id = p_rider_id;

    GET DIAGNOSTICS v_rows_affected = ROW_COUNT;
    RETURN v_rows_affected > 0;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_delete_rider(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    deleted_count INT;
    v_scope UUID[];
    v_riders UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id, true);
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

    DELETE FROM tracking.riders r WHERE r.id = p_rider_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$function$;
