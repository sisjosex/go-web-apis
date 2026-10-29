CREATE OR REPLACE FUNCTION tenancy.sp_tenant_staff_users(p_tenant_id uuid)
 RETURNS TABLE(user_id uuid)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT tu.user_id
    FROM tenancy.tenant_users tu
    WHERE tu.tenant_id = p_tenant_id
      AND tu.is_active
      AND tu.role IN ('owner', 'admin', 'member');
$function$;

COMMENT ON FUNCTION tenancy.sp_tenant_staff_users(uuid) IS 'The tenant''s active staff — owner, admin, member, the roles that see the alerts page — the accounts an operational alert is pushed to (TRACK-071 D3)';
