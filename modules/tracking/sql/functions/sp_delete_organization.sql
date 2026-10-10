CREATE OR REPLACE FUNCTION tracking.sp_delete_organization(p_tenant_id uuid, p_organization_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.riders r WHERE r.organization_id = p_organization_id
    ) THEN
        RAISE EXCEPTION 'organization.has-riders' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$function$;
