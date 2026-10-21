CREATE OR REPLACE FUNCTION tracking.sp_trip_task_proof_key(p_tenant_id uuid, p_task_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS character varying
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope          UUID[];
    v_riders         UUID[];
    v_guardian_scope UUID[];
    v_key            VARCHAR;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    END IF;
    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    SELECT CAST(k.proof->>'photo' AS VARCHAR) INTO v_key
    FROM tracking.trip_stop_tasks k
    INNER JOIN tracking.trip_stops ts ON ts.id = k.trip_stop_id
    INNER JOIN tracking.trips t ON t.id = ts.trip_id AND t.tenant_id = p_tenant_id
    INNER JOIN tracking.riders r ON k.subject_type = 'passenger' AND r.id = k.subject_id
    WHERE k.id = p_task_id
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
      AND (v_riders IS NULL OR r.id = ANY(v_riders))
      AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope));

    IF v_key IS NULL THEN
        RAISE EXCEPTION 'trip.task.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_key;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_task_proof_key(uuid,uuid,uuid,uuid) IS 'The storage key of a task''s drop-off proof photo (MOBILE-032), scoped as sp_get_rider_status: to p_scope_user_id''s organizations and to p_guardian_user_id''s own riders when either is set; raises trip.task.not-found when the task has no photo or is out of scope';
