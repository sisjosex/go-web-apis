CREATE OR REPLACE FUNCTION tenancy.sp_revoke_user_role(p_tenant_id uuid, p_requester_user_id uuid, p_user_id uuid, p_role_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate requester is owner or admin in this tenant
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.tenant_users tu
        WHERE tu.user_id   = p_requester_user_id
          AND tu.tenant_id = p_tenant_id
          AND tu.is_active = TRUE
          AND tu.role IN ('owner', 'admin')
    ) THEN
        RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tenancy.user_roles ur
    WHERE ur.user_id   = p_user_id
      AND ur.role_id   = p_role_id
      AND ur.tenant_id = p_tenant_id;

    RETURN TRUE;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_revoke_user_role(uuid,uuid,uuid,uuid) IS 'Revokes a role from a tenant user; idempotent if the role was not assigned';
