CREATE OR REPLACE FUNCTION tracking.fn_supervised_riders(p_tenant_id uuid, p_user_id uuid)
 RETURNS uuid[]
 LANGUAGE sql
 STABLE
AS $function$
    -- NULL unless one of the user's memberships carries groups: then the riders of each membership —
    -- all of an organization joined without groups, only the given groups' of one joined with them.
    SELECT CASE WHEN bool_or(om.groups IS NOT NULL) THEN (
        SELECT COALESCE(ARRAY_AGG(r.id), ARRAY[]::UUID[])
        FROM tracking.organization_members m
        INNER JOIN tracking.organizations mo ON mo.id = m.organization_id AND mo.tenant_id = p_tenant_id
        INNER JOIN tracking.riders r ON r.organization_id = m.organization_id
        WHERE m.user_id = p_user_id
          AND (m.groups IS NULL OR r.group_label = ANY(m.groups))
    ) END
    FROM tracking.organization_members om
    INNER JOIN tracking.organizations o ON o.id = om.organization_id
    WHERE om.user_id = p_user_id
      AND o.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.fn_supervised_riders(uuid,uuid) IS 'The riders a supervisor with groups may see — NULL when no membership of the user carries groups, so the organization scope alone applies (TRACK-067 D2)';
