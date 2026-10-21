CREATE OR REPLACE FUNCTION tracking.sp_get_organization(p_tenant_id uuid, p_organization_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, kind character varying, name character varying, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, absence_cutoff_min integer, latitude numeric, longitude numeric, address character varying, stop_place_id uuid, empty_trip_cancel_min integer, delay_notice_min integer, my_role character varying, my_groups text[])
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_role   VARCHAR;
    v_groups TEXT[];
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_scope_user_id IS NOT NULL THEN
        SELECT m.role, m.groups INTO v_role, v_groups
        FROM tracking.organization_members m
        WHERE m.organization_id = p_organization_id AND m.user_id = p_scope_user_id;
        -- Another organization is as absent to an organization user as one that does not exist.
        IF v_role IS NULL THEN
            RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    SELECT fr.*, v_role, v_groups
    FROM tracking.fn_organization_row(p_tenant_id, p_organization_id) fr;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_organization(uuid,uuid,uuid) IS 'One organization; for an organization user (p_scope_user_id) only one it belongs to, with my_role and my_groups, else organization.not-found (TRACK-067)';
