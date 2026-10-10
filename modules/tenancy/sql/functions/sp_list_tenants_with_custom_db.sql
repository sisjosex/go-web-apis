CREATE OR REPLACE FUNCTION tenancy.sp_list_tenants_with_custom_db()
 RETURNS TABLE(id uuid, slug character varying, name character varying, database_url text, is_active boolean, is_suspended boolean, created_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        CAST(t.id AS UUID),
        CAST(t.slug AS VARCHAR),
        CAST(t.name AS VARCHAR),
        CAST(t.database_url AS TEXT),
        t.is_active,
        t.is_suspended,
        t.created_at
    FROM tenancy.tenants t
    WHERE t.database_url IS NOT NULL AND TRIM(t.database_url) != ''
    ORDER BY t.slug ASC;
END;
$function$;
