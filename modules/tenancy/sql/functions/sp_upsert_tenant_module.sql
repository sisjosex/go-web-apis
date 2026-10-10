CREATE OR REPLACE FUNCTION tenancy.sp_upsert_tenant_module(p_tenant_id uuid, p_module_code character varying, p_is_enabled boolean, p_config jsonb, p_enabled_by uuid)
 RETURNS TABLE(module_code character varying, is_enabled boolean, config jsonb, enabled_at timestamp with time zone, enabled_by uuid)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF p_tenant_id IS NULL THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_module_code IS NULL OR TRIM(p_module_code) = '' THEN
        RAISE EXCEPTION 'tenant.module.code-required' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tenancy.tenant_modules AS tm (tenant_id, module_code, is_enabled, config, enabled_by)
    VALUES (p_tenant_id, LOWER(TRIM(p_module_code)), p_is_enabled, COALESCE(p_config, '{}'), p_enabled_by)
    ON CONFLICT ON CONSTRAINT uq_tenant_module DO UPDATE
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
$function$;
