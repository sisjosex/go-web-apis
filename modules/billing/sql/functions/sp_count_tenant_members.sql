CREATE OR REPLACE FUNCTION billing.sp_count_tenant_members(p_tenant_id uuid)
 RETURNS bigint
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_count BIGINT := 0;
BEGIN
    IF to_regclass('tenancy.tenant_users') IS NOT NULL THEN
        SELECT COUNT(*) INTO v_count FROM tenancy.tenant_users tu WHERE tu.tenant_id = p_tenant_id AND tu.is_active;
    END IF;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION billing.sp_count_tenant_members(uuid) IS 'Active members of a business, its users_per_tenant usage (BILLING-001)';
