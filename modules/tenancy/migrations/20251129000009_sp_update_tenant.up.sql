-- Stored procedure to update tenant
-- Usage: SELECT * FROM tenancy.sp_update_tenant('tenant-uuid', 'New Name', NULL, NULL, '{"key":"value"}');
CREATE OR REPLACE FUNCTION tenancy.sp_update_tenant(
    p_tenant_id UUID,
    p_name VARCHAR DEFAULT NULL,
    p_database_url TEXT DEFAULT NULL,
    p_schema_name VARCHAR DEFAULT NULL,
    p_settings JSONB DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    slug VARCHAR,
    name VARCHAR,
    database_url TEXT,
    schema_name VARCHAR,
    is_active BOOLEAN,
    is_suspended BOOLEAN,
    suspended_reason TEXT,
    settings JSONB,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
) LANGUAGE plpgsql AS $$
BEGIN
    -- Check if tenant exists
    IF NOT EXISTS (SELECT 1 FROM tenancy.tenants t WHERE t.id = p_tenant_id) THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
    END IF;

    -- Update tenant (only fields that are not NULL)
    UPDATE tenancy.tenants t
    SET 
        name = COALESCE(p_name, t.name),
        database_url = COALESCE(p_database_url, t.database_url),
        schema_name = COALESCE(p_schema_name, t.schema_name),
        settings = COALESCE(p_settings, t.settings)
    WHERE t.id = p_tenant_id;

    -- Return updated tenant
    RETURN QUERY
    SELECT 
        t.id,
        t.slug,
        t.name,
        t.database_url,
        t.schema_name,
        t.is_active,
        t.is_suspended,
        t.suspended_reason,
        t.settings,
        t.created_at,
        t.updated_at
    FROM tenancy.tenants t
    WHERE t.id = p_tenant_id;
END;
$$;
