CREATE OR REPLACE FUNCTION tenancy.sp_merge_tenant_settings(p_tenant_id uuid, p_patch jsonb)
 RETURNS jsonb
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_settings JSONB;
BEGIN
    UPDATE tenancy.tenants t
    SET settings = COALESCE(t.settings, '{}'::jsonb) || COALESCE(p_patch, '{}'::jsonb)
    WHERE t.id = p_tenant_id
    RETURNING t.settings INTO v_settings;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
    END IF;

    RETURN v_settings;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_merge_tenant_settings(uuid,jsonb) IS 'Merges the keys sent into a tenant''s settings, keeping the rest, and answers the result (TENANCY-007 D1); the API validates keys and values first';
