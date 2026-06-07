CREATE OR REPLACE FUNCTION users.sp_get_user_stats(
    p_tenant_id       UUID,
    p_exclude_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    total_count    BIGINT,
    active_count   BIGINT,
    inactive_count BIGINT,
    expired_count  BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT
        COUNT(*)::BIGINT AS total_count,
        COUNT(*) FILTER (
            WHERE u.is_active = TRUE
              AND (u.expiration_date IS NULL OR u.expiration_date > CURRENT_DATE)
        )::BIGINT AS active_count,
        COUNT(*) FILTER (
            WHERE u.is_active = FALSE
        )::BIGINT AS inactive_count,
        COUNT(*) FILTER (
            WHERE u.expiration_date IS NOT NULL
              AND u.expiration_date <= CURRENT_DATE
        )::BIGINT AS expired_count
    FROM auth.users u
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id
    WHERE u.deleted_at IS NULL
      AND (p_exclude_user_id IS NULL OR u.id != p_exclude_user_id);
END;
$$;

COMMENT ON FUNCTION users.sp_get_user_stats(UUID, UUID) IS
'Returns total, active, inactive and expired user counts for a tenant in a single query.';
