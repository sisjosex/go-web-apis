-- TRACK-016 step 2 (D1): who a tenant's operational digests go to.
--
-- Owners and admins, and only those: a digest naming every expiring document is an operator's job
-- list, not something a member or a mobile-only account has any use for. The access levels
-- organization, portal and driver are excluded by not being in the list, so a level added later is
-- silently left out rather than silently included.
--
-- This lives in tenancy because tenant_users and auth.users are the platform database's; the
-- documents themselves are read from each tenant's own database by the job that calls this first.

CREATE FUNCTION tenancy.sp_list_tenant_admin_emails(
    p_tenant_id UUID
)
RETURNS TABLE(
    user_id UUID,
    email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        u.id,
        CAST(u.email AS VARCHAR),
        CAST(u.first_name AS VARCHAR),
        CAST(u.last_name AS VARCHAR)
    FROM tenancy.tenant_users tu
    INNER JOIN auth.users u ON u.id = tu.user_id
    WHERE tu.tenant_id = p_tenant_id
      AND tu.is_active = TRUE
      AND tu.role IN ('owner', 'admin')
      AND u.deleted_at IS NULL
      AND u.email IS NOT NULL
    ORDER BY u.email ASC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tenancy.sp_list_tenant_admin_emails(UUID) IS
'The active owners and admins of one tenant, with their email — the recipients of an operational digest (TRACK-016 D1)';
