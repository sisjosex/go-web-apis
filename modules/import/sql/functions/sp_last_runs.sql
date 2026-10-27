CREATE OR REPLACE FUNCTION import.sp_last_runs(p_tenant_id uuid)
 RETURNS TABLE(resource character varying, last_run_at timestamp with time zone)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT r.resource, MAX(r.created_at)
    FROM import.runs r
    WHERE r.tenant_id = p_tenant_id
    GROUP BY r.resource
$function$;

COMMENT ON FUNCTION import.sp_last_runs(uuid) IS 'When each resource was last imported in a tenant — the page''s "Orden sugerido" ticks (IMPORT-002)';
