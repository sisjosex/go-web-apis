-- Migration: sp_list_tenants_with_custom_db
-- Module: tenancy
-- Created: 2025-12-03 20:30:37

-- Create stored procedure to list all tenants with custom databases
CREATE OR REPLACE FUNCTION tenancy.sp_list_tenants_with_custom_db()
RETURNS TABLE (
    id UUID,
    slug VARCHAR,
    name VARCHAR,
    description TEXT,
    database_url TEXT,
    is_active BOOLEAN,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        t.id,
        t.slug,
        t.name,
        t.description,
        t.database_url,
        t.is_active,
        t.created_at,
        t.updated_at
    FROM tenancy.tenants t
    WHERE t.database_url IS NOT NULL
        AND t.database_url != ''
        AND t.is_active = true
    ORDER BY t.name ASC;
END;
$$ LANGUAGE plpgsql;

