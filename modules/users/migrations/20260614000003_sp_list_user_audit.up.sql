CREATE FUNCTION users.sp_list_user_audit(
    p_tenant_id UUID,
    p_page      INT         DEFAULT 1,
    p_limit     INT         DEFAULT 20,
    p_search    VARCHAR     DEFAULT NULL,
    p_action    VARCHAR(30) DEFAULT NULL,
    p_from      TIMESTAMPTZ DEFAULT NULL,
    p_to        TIMESTAMPTZ DEFAULT NULL
) RETURNS TABLE (
    id                 UUID,
    action             VARCHAR,
    target_user_id     UUID,
    target_user_email  VARCHAR,
    performed_by       UUID,
    performed_by_email VARCHAR,
    metadata           JSONB,
    source             VARCHAR,
    created_at         TIMESTAMPTZ,
    total              BIGINT
) AS $$
DECLARE
    l_offset INT := (p_page - 1) * p_limit;
BEGIN
    RETURN QUERY
    SELECT
        al.id,
        CAST(al.action AS VARCHAR),
        al.target_user_id,
        CAST(tu.email AS VARCHAR),
        al.performed_by,
        CAST(pu.email AS VARCHAR),
        al.metadata,
        CAST(al.source AS VARCHAR),
        al.created_at,
        COUNT(*) OVER () AS total
    FROM users.user_audit_log al
    LEFT JOIN auth.users tu ON tu.id = al.target_user_id
    LEFT JOIN auth.users pu ON pu.id = al.performed_by
    WHERE al.tenant_id = p_tenant_id
      AND (p_action IS NULL OR al.action = p_action)
      AND (p_from IS NULL OR al.created_at >= p_from)
      AND (p_to IS NULL OR al.created_at <= p_to)
      AND (
          p_search IS NULL
          OR CAST(tu.email AS TEXT) ILIKE '%' || p_search || '%'
          OR CAST(pu.email AS TEXT) ILIKE '%' || p_search || '%'
      )
    ORDER BY al.created_at DESC
    LIMIT p_limit
    OFFSET l_offset;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION users.sp_list_user_audit IS 'Returns paginated audit log entries scoped to a tenant with optional filtering';
