CREATE OR REPLACE FUNCTION tenancy.sp_create_role(p_tenant_id uuid, p_requester_user_id uuid, p_name character varying, p_description text)
 RETURNS TABLE(id uuid, tenant_id uuid, name character varying, description text, is_system boolean, created_at timestamp with time zone, updated_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tenancy.tenant_users tu
        WHERE tu.user_id   = p_requester_user_id
          AND tu.tenant_id = p_tenant_id
          AND tu.is_active = TRUE
          AND tu.role IN ('owner', 'admin')
    ) THEN
        RAISE EXCEPTION 'tenant.user.unauthorized' USING ERRCODE = 'P0001';
    END IF;

    IF TRIM(p_name) = '' OR p_name IS NULL THEN
        RAISE EXCEPTION 'tenant.role.validation-failed' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tenancy.roles AS r (tenant_id, name, description)
    VALUES (p_tenant_id, TRIM(p_name), p_description)
    RETURNING
        r.id,
        r.tenant_id,
        CAST(r.name        AS VARCHAR),
        r.description,
        r.is_system,
        r.created_at,
        r.updated_at;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_create_role(uuid,uuid,character varying,text) IS 'Creates a custom role scoped to a tenant; requires requester to be owner or admin';
