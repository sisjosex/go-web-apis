CREATE OR REPLACE FUNCTION import.sp_record_run(p_tenant_id uuid, p_resource character varying, p_file_name character varying, p_total integer, p_created integer, p_updated integer, p_skipped integer, p_failed integer, p_warnings integer, p_performed_by uuid)
 RETURNS uuid
 LANGUAGE sql
AS $function$
    INSERT INTO import.runs (tenant_id, resource, file_name, total, created, updated, skipped, failed, warnings, performed_by)
    VALUES (p_tenant_id, p_resource, NULLIF(LEFT(p_file_name, 255), ''), p_total, p_created, p_updated, p_skipped, p_failed, p_warnings, p_performed_by)
    RETURNING id
$function$;

COMMENT ON FUNCTION import.sp_record_run(uuid,character varying,character varying,integer,integer,integer,integer,integer,integer,uuid) IS 'Writes one confirmed import''s counts and answers its id (IMPORT-002)';
