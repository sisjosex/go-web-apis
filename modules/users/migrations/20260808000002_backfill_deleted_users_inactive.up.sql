-- Backfill: users soft-deleted before USERS-009 kept is_active = TRUE, which
-- left them able to log in and visible to every module that does not filter
-- deleted_at. Bring them in line with what sp_soft_delete_user now does.

UPDATE auth.users
SET is_active = FALSE,
    updated_at = NOW()
WHERE deleted_at IS NOT NULL
  AND is_active = TRUE;
