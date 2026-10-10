CREATE OR REPLACE FUNCTION tracking.sp_claim_optimization_run(p_tenant_id uuid, p_run_id uuid)
 RETURNS TABLE(input jsonb)
 LANGUAGE sql
AS $function$
    UPDATE tracking.optimization_runs r
       SET status = 'running', updated_at = now()
     WHERE r.id = p_run_id AND r.tenant_id = p_tenant_id AND r.status IN ('queued', 'running')
    RETURNING r.input;
$function$;

COMMENT ON FUNCTION tracking.sp_claim_optimization_run(uuid,uuid) IS 'Marks a queued (or interrupted running) run as running and answers its problem; no row once it is done or failed (TRACK-014 step 2)';
