CREATE OR REPLACE FUNCTION tracking.sp_get_invited_guardian(p_tenant_id uuid, p_user_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(user_id uuid, first_name character varying, email character varying, phone character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    -- Sending an invitation acts on the riders' families: an organization user needs to administer them.
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id, true);
    END IF;

    RETURN QUERY
    SELECT rc.user_id,
           CAST(COALESCE(MAX(u.first_name), MAX(rc.name)) AS VARCHAR),
           CAST(COALESCE(MAX(u.email), MAX(rc.email)) AS VARCHAR),
           CAST(COALESCE(MAX(rc.phone), MAX(u.phone)) AS VARCHAR)
    FROM tracking.rider_contacts rc
    INNER JOIN tracking.riders r ON r.id = rc.rider_id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN auth.users u ON u.id = rc.user_id AND u.deleted_at IS NULL
    WHERE rc.user_id = p_user_id
      AND rc.relation = 'guardian'
      AND o.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
    GROUP BY rc.user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'invite.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_invited_guardian(uuid,uuid,uuid) IS 'The guardian account an invitation is sent or resent to: its first name, email and phone; raises invite.not-found when it guards no rider of the tenant or of the organizations p_scope_user_id administers (TRACK-062)';
