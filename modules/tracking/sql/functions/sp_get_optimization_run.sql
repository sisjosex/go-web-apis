CREATE OR REPLACE FUNCTION tracking.sp_get_optimization_run(p_tenant_id uuid, p_run_id uuid)
 RETURNS TABLE(id uuid, organization_id uuid, status character varying, error_code character varying, params jsonb, result jsonb, applied_at timestamp with time zone, created_at timestamp with time zone)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT r.id, r.organization_id, r.status, r.error_code, r.params, r.result, r.applied_at, r.created_at
    FROM tracking.optimization_runs r
    WHERE r.id = p_run_id AND r.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'optimization.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_optimization_run(uuid,uuid) IS 'One run: its state, params, result and when it was applied; raises optimization.not-found (TRACK-014 step 2)';
