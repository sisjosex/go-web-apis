-- Backfill: memberships of users soft-deleted before USERS-009 stayed active,
-- which is why deleted users kept showing up in Roles and permissions
-- (tenancy.sp_list_role_users filters tenant_users.is_active, not users.deleted_at).
--
-- Lives in tenancy rather than users because migrations run in ENABLED_MODULES
-- order (users before tenancy) and because tenant databases have no tenancy schema.

UPDATE tenancy.tenant_users tu
SET is_active = FALSE
WHERE tu.is_active = TRUE
  AND EXISTS (
      SELECT 1
      FROM auth.users u
      WHERE u.id = tu.user_id
        AND u.deleted_at IS NOT NULL
  );
