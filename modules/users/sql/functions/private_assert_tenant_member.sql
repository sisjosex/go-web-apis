CREATE OR REPLACE FUNCTION users.private_assert_tenant_member(p_tenant_id uuid, p_user_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF to_regclass('tenancy.tenant_users') IS NULL THEN
        RETURN;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM tenancy.tenant_users tu
        WHERE tu.tenant_id = p_tenant_id
          AND tu.user_id   = p_user_id
          AND tu.is_active = TRUE
    ) THEN
        RAISE EXCEPTION 'user.not-found'
            USING ERRCODE = 'U0002',
                  DETAIL = 'User not found in this workspace';
    END IF;
END;
$function$;

COMMENT ON FUNCTION users.private_assert_tenant_member(uuid,uuid) IS 'Raises user.not-found unless the user is an active member of the tenant. No-op on tenant databases.';
