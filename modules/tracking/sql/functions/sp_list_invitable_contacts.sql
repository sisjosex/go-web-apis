CREATE OR REPLACE FUNCTION tracking.sp_list_invitable_contacts(p_tenant_id uuid, p_organization_id uuid DEFAULT NULL::uuid, p_rider_ids uuid[] DEFAULT NULL::uuid[], p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(contact_id uuid, rider_id uuid, rider_name character varying, name character varying, phone character varying, email character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope  UUID[];
BEGIN
    IF p_organization_id IS NULL AND p_rider_ids IS NULL THEN
        RAISE EXCEPTION 'invite.target-required' USING ERRCODE = 'P0001';
    END IF;
    -- Inviting writes the riders' families: an organization user needs to administer them (TRACK-067).
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id, true);
        IF p_organization_id IS NOT NULL AND NOT (p_organization_id = ANY(v_scope)) THEN
            RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    SELECT rc.id, r.id,
           CAST(r.first_name || ' ' || r.last_name AS VARCHAR),
           CAST(rc.name AS VARCHAR),
           CAST(NULLIF(TRIM(rc.phone), '') AS VARCHAR),
           CAST(NULLIF(LOWER(TRIM(rc.email)), '') AS VARCHAR)
    FROM tracking.rider_contacts rc
    INNER JOIN tracking.riders r ON r.id = rc.rider_id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE o.tenant_id = p_tenant_id
      AND rc.relation = 'guardian'
      AND rc.user_id IS NULL
      AND r.status = 'active'
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
      AND (p_rider_ids IS NULL OR r.id = ANY(p_rider_ids))
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
    -- One person is often the guardian of siblings: their rows come together, so one grant serves all.
    ORDER BY LOWER(rc.email), rc.phone, r.last_name, r.first_name, rc.id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_invitable_contacts(uuid,uuid,uuid[],uuid) IS 'The guardian contacts without an account of an organization''s or the given active riders, each with its rider, ordered so one person''s rows are adjacent; an organization user only in organizations it administers; raises invite.target-required (TRACK-062)';
