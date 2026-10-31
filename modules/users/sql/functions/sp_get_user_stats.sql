CREATE OR REPLACE FUNCTION users.sp_get_user_stats(p_tenant_id uuid, p_exclude_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(total_count bigint, active_count bigint, inactive_count bigint, expired_count bigint, by_role jsonb, pending_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    WITH people AS (
        SELECT
            u.id,
            u.is_active,
            u.expiration_date,
            tu.role,
            NOT EXISTS (SELECT 1 FROM auth.user_sessions us WHERE us.user_id = u.id) AS pending
        FROM auth.users u
        INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
        WHERE u.deleted_at IS NULL
          AND tu.role NOT IN ('driver', 'portal')
          AND (p_exclude_user_id IS NULL OR u.id != p_exclude_user_id)
    )
    SELECT
        COUNT(*)::BIGINT,
        COUNT(*) FILTER (
            WHERE pe.is_active = TRUE
              AND (pe.expiration_date IS NULL OR pe.expiration_date > CURRENT_DATE)
        )::BIGINT,
        COUNT(*) FILTER (
            WHERE pe.is_active = FALSE
        )::BIGINT,
        COUNT(*) FILTER (
            WHERE pe.expiration_date IS NOT NULL
              AND pe.expiration_date <= CURRENT_DATE
        )::BIGINT,
        jsonb_build_array(
            jsonb_build_object('key', 'admin', 'count', COUNT(*) FILTER (WHERE pe.role IN ('owner', 'admin'))),
            jsonb_build_object('key', 'organization', 'count', COUNT(*) FILTER (WHERE pe.role = 'organization'))
        ) || (
            SELECT COALESCE(jsonb_agg(jsonb_build_object('key', ro.id, 'count', (
                SELECT COUNT(*)
                FROM tenancy.user_roles ur
                INNER JOIN tenancy.tenant_users tu ON tu.user_id = ur.user_id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
                INNER JOIN auth.users u ON u.id = ur.user_id AND u.deleted_at IS NULL
                WHERE ur.role_id = ro.id AND ur.tenant_id = p_tenant_id AND tu.role NOT IN ('driver', 'portal')
                  AND (p_exclude_user_id IS NULL OR ur.user_id != p_exclude_user_id)
            )) ORDER BY ro.name), '[]'::jsonb)
            FROM tenancy.roles ro
            WHERE ro.tenant_id = p_tenant_id
        ),
        COUNT(*) FILTER (WHERE pe.pending)::BIGINT
    FROM people pe;
END;
$function$;

COMMENT ON FUNCTION users.sp_get_user_stats(uuid,uuid) IS 'Returns a tenant''s panel people in one query: total, active, inactive and expired counts, how many hold each role (admin, organization, then one per role id) and how many never signed in. App-only portal and driver accounts are not counted (USERS-015).';
