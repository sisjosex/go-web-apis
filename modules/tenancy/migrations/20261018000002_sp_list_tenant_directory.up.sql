-- INFRA-009 D5: every active tenant and the database it lives in — the background workers' directory.
-- database_url NULL is the shared platform database (INFRA-006 D2); the walkers group by it.

CREATE FUNCTION tenancy.sp_list_tenant_directory()
RETURNS TABLE (
    id UUID,
    slug VARCHAR,
    name VARCHAR,
    database_url TEXT
) LANGUAGE sql STABLE AS $$
    SELECT
        t.id,
        CAST(t.slug AS VARCHAR),
        CAST(t.name AS VARCHAR),
        CAST(NULLIF(TRIM(t.database_url), '') AS TEXT)
    FROM tenancy.tenants t
    WHERE t.is_active
    ORDER BY t.slug ASC;
$$;

COMMENT ON FUNCTION tenancy.sp_list_tenant_directory() IS
'Every active tenant with the database it lives in, NULL for the shared platform database (INFRA-009)';
