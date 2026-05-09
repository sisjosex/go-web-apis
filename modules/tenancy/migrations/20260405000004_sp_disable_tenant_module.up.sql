CREATE OR REPLACE FUNCTION tenancy.sp_disable_tenant_module(
    p_tenant_id   UUID,
    p_module_code VARCHAR
)
RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    IF p_tenant_id IS NULL THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tenancy.tenant_modules
    SET is_enabled = FALSE,
        updated_at = CURRENT_TIMESTAMP
    WHERE tenant_id = p_tenant_id
      AND module_code = LOWER(TRIM(p_module_code));

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant.module.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$;
