CREATE OR REPLACE FUNCTION tenancy.sp_revoke_app_access(p_tenant_id uuid, p_user_id uuid, p_level character varying)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count INT;
BEGIN
    UPDATE tenancy.tenant_users tu
    SET is_active = FALSE
    WHERE tu.tenant_id = p_tenant_id
      AND tu.user_id = p_user_id
      AND tu.role = p_level
      AND p_level IN ('portal', 'driver', 'organization');
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count > 0;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_revoke_app_access(uuid,uuid,character varying) IS 'Deactivates an app-level membership (portal, driver, organization) once nothing in tracking links the account (TRACK-032)';
