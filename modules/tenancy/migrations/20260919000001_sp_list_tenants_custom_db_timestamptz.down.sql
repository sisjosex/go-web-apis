-- Restores the TIMESTAMP signature from 20260307140000. The function is broken
-- in that shape — this exists only so the migration is reversible.

DROP FUNCTION IF EXISTS tenancy.sp_list_tenants_with_custom_db() CASCADE;

CREATE FUNCTION tenancy.sp_list_tenants_with_custom_db()
RETURNS TABLE (
    id UUID,
    slug VARCHAR,
    name VARCHAR,
    database_url TEXT,
    is_active BOOLEAN,
    is_suspended BOOLEAN,
    created_at TIMESTAMP
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
