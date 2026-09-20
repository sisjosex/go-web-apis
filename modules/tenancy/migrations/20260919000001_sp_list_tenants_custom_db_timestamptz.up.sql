-- sp_list_tenants_with_custom_db returned created_at as TIMESTAMP while
-- tenancy.tenants.created_at is TIMESTAMPTZ. It is the one column the body does
-- not CAST, so PostgreSQL compared the declared signature against the actual row
-- type and aborted every call with SQLSTATE 42804 — `cli tenant -list` and
-- `cli tenant -migrate all` never worked on any database.
--
-- Signature change ⇒ DROP + CREATE, never CREATE OR REPLACE.

DROP FUNCTION IF EXISTS tenancy.sp_list_tenants_with_custom_db() CASCADE;

CREATE FUNCTION tenancy.sp_list_tenants_with_custom_db()
RETURNS TABLE (
    id UUID,
    slug VARCHAR,
    name VARCHAR,
    database_url TEXT,
    is_active BOOLEAN,
    is_suspended BOOLEAN,
    created_at TIMESTAMPTZ
) LANGUAGE plpgsql AS $$
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
$$;
