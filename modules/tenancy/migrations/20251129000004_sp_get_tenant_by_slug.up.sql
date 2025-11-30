-- Stored procedure to get tenant by slug (public data only, for middleware)
-- Usage: SELECT * FROM tenancy.sp_get_tenant_by_slug('acme-corp');
CREATE OR REPLACE FUNCTION tenancy.sp_get_tenant_by_slug(
    p_slug VARCHAR
)
RETURNS TABLE (
    id UUID,
    slug VARCHAR,
    name VARCHAR,
    database_url TEXT,
    schema_name VARCHAR,
    is_active BOOLEAN,
    is_suspended BOOLEAN,
    created_at TIMESTAMP WITH TIME ZONE
) LANGUAGE plpgsql AS $$
DECLARE
    v_slug VARCHAR;
BEGIN
    v_slug := LOWER(TRIM(p_slug));
    
    IF v_slug = '' OR v_slug IS NULL THEN
        RAISE EXCEPTION 'tenant.slug.required' USING ERRCODE = 'T0001';
    END IF;

    RETURN QUERY
    SELECT 
        t.id,
        t.slug,
        t.name,
        t.database_url,
        t.schema_name,
        t.is_active,
        t.is_suspended,
        t.created_at
    FROM tenancy.tenants t
    WHERE t.slug = v_slug;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
    END IF;
END;
$$;
