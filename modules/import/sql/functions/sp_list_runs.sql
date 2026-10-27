CREATE OR REPLACE FUNCTION import.sp_list_runs(p_tenant_id uuid, p_resource character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, resource character varying, file_name character varying, total integer, created integer, updated integer, skipped integer, failed integer, performed_by_name character varying, created_at timestamp with time zone, total_count bigint)
 LANGUAGE sql
 STABLE
AS $function$
    WITH f AS (
        SELECT r.*
        FROM import.runs r
        WHERE r.tenant_id = p_tenant_id
          AND (p_resource IS NULL OR r.resource = p_resource)
    ), pg AS (
        SELECT f.*, COUNT(*) OVER () AS total_count
        FROM f
        ORDER BY f.created_at DESC
        LIMIT GREATEST(p_page_size, 1) OFFSET (GREATEST(p_page, 1) - 1) * GREATEST(p_page_size, 1)
    )
    SELECT pg.id, pg.resource, pg.file_name, pg.total, pg.created, pg.updated, pg.skipped, pg.failed,
           CAST(NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '') AS VARCHAR) AS performed_by_name,
           pg.created_at, pg.total_count
    FROM pg
    LEFT JOIN auth.users u ON u.id = pg.performed_by
    ORDER BY pg.created_at DESC
$function$;

COMMENT ON FUNCTION import.sp_list_runs(uuid,character varying,integer,integer) IS 'A tenant''s imports, newest first, with who ran each; one resource when p_resource is set (IMPORT-002)';
