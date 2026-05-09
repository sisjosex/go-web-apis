CREATE OR REPLACE FUNCTION tenancy.sp_upsert_tenant_module(
    p_tenant_id   UUID,
    p_module_code VARCHAR,
    p_is_enabled  BOOLEAN,
    p_config      JSONB,
    p_enabled_by  UUID
)
RETURNS TABLE (
    module_code VARCHAR,
    is_enabled  BOOLEAN,
    config      JSONB,
    enabled_at  TIMESTAMP WITH TIME ZONE,
    enabled_by  UUID
) LANGUAGE plpgsql AS $$
BEGIN
    IF p_tenant_id IS NULL THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_module_code IS NULL OR TRIM(p_module_code) = '' THEN
        RAISE EXCEPTION 'tenant.module.code-required' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tenancy.tenant_modules (tenant_id, module_code, is_enabled, config, enabled_by)
    VALUES (p_tenant_id, LOWER(TRIM(p_module_code)), p_is_enabled, COALESCE(p_config, '{}'), p_enabled_by)
    ON CONFLICT (tenant_id, module_code) DO UPDATE
        SET is_enabled = EXCLUDED.is_enabled,
            config     = EXCLUDED.config,
            enabled_by = EXCLUDED.enabled_by,
            updated_at = CURRENT_TIMESTAMP;

    RETURN QUERY
    SELECT
        CAST(tm.module_code AS VARCHAR),
        tm.is_enabled,
        tm.config,
        tm.enabled_at,
        tm.enabled_by
    FROM tenancy.tenant_modules tm
    WHERE tm.tenant_id = p_tenant_id
      AND tm.module_code = LOWER(TRIM(p_module_code));
END;
$$;
