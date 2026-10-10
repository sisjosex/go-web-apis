CREATE OR REPLACE FUNCTION tracking.sp_finish_optimization_run(p_tenant_id uuid, p_run_id uuid, p_status character varying, p_error_code character varying DEFAULT NULL::character varying, p_result jsonb DEFAULT NULL::jsonb)
 RETURNS void
 LANGUAGE sql
AS $function$
    UPDATE tracking.optimization_runs r
       SET status = p_status, error_code = p_error_code, result = p_result, updated_at = now()
     WHERE r.id = p_run_id AND r.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.sp_finish_optimization_run(uuid,uuid,character varying,character varying,jsonb) IS 'Records a run''s outcome: done with its routes and unassigned riders, or failed with an error code (TRACK-014 step 2)';
