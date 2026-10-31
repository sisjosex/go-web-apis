CREATE OR REPLACE FUNCTION tenancy.sp_seed_operator_role(p_tenant_id uuid, p_permissions text[])
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_role_id UUID;
BEGIN
    INSERT INTO tenancy.roles (tenant_id, name, description)
    VALUES (p_tenant_id, 'Operador', 'El día a día de taypi go')
    RETURNING roles.id INTO v_role_id;

    INSERT INTO tenancy.role_permissions (role_id, permission_code)
    SELECT v_role_id, code
    FROM unnest(COALESCE(p_permissions, ARRAY[]::text[])) AS code;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_seed_operator_role(uuid,text[]) IS 'Gives a new business its editable Operador role holding p_permissions (USERS-015 D1). Runs in the statement that creates the tenant, so a business never exists without it.';
