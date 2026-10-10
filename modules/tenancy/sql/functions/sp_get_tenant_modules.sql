CREATE OR REPLACE FUNCTION tenancy.sp_get_tenant_modules(p_tenant_id uuid)
 RETURNS TABLE(module_code character varying, is_enabled boolean, config jsonb, enabled_at timestamp with time zone, enabled_by uuid)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF p_tenant_id IS NULL THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(tm.module_code AS VARCHAR),
        tm.is_enabled,
        tm.config,
        tm.enabled_at,
        tm.enabled_by
    FROM tenancy.tenant_modules tm
    WHERE tm.tenant_id = p_tenant_id
    ORDER BY tm.module_code ASC;
END;
$function$;
