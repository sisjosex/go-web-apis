CREATE OR REPLACE FUNCTION tracking.fn_organization_scope(p_tenant_id uuid, p_user_id uuid)
 RETURNS uuid[]
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    SELECT ARRAY_AGG(om.organization_id)
    INTO v_scope
    FROM tracking.organization_members om
    -- The join is the tenant guard: a membership in another tenant's organization is not a scope here.
    INNER JOIN tracking.organizations o ON o.id = om.organization_id
    WHERE om.user_id = p_user_id
      AND o.tenant_id = p_tenant_id;

    IF v_scope IS NULL OR array_length(v_scope, 1) IS NULL THEN
        RAISE EXCEPTION 'tracking.organization.scope-denied'
            USING ERRCODE = 'P0001', DETAIL = 'User belongs to no organization in this tenant';
    END IF;

    RETURN v_scope;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_organization_scope(uuid,uuid) IS 'The organizations a user may see inside one tenant (TRACK-015 D1); raises tracking.organization.scope-denied when they belong to none';
