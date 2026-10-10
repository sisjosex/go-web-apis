CREATE OR REPLACE FUNCTION tracking.sp_delete_organization_member(p_tenant_id uuid, p_organization_id uuid, p_user_id uuid)
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

    DELETE FROM tracking.organization_members m
    WHERE m.organization_id = p_organization_id AND m.user_id = p_user_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;

    IF deleted_count = 0 THEN
        RAISE EXCEPTION 'organization-member.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN true;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_organization_member(uuid,uuid,uuid) IS 'Takes a user off an organization; raises tracking.organization-member.not-found when they were not on it, and leaves the user account untouched';
