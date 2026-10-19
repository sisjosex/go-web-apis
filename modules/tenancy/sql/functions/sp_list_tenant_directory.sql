CREATE OR REPLACE FUNCTION tenancy.sp_list_tenant_directory()
 RETURNS TABLE(id uuid, slug character varying, name character varying, database_url text, settings jsonb)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        t.id,
        CAST(t.slug AS VARCHAR),
        CAST(t.name AS VARCHAR),
        CAST(NULLIF(TRIM(t.database_url), '') AS TEXT),
        COALESCE(t.settings, '{}'::jsonb)
    FROM tenancy.tenants t
    WHERE t.is_active
    ORDER BY t.slug ASC;
$function$;

COMMENT ON FUNCTION tenancy.sp_list_tenant_directory() IS 'Every active tenant with the database it lives in, NULL for the shared platform database (INFRA-009), and its settings for the jobs that read one (TRACK-063)';
