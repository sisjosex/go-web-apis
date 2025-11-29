-- Remove soft delete columns from users table
ALTER TABLE auth.users DROP COLUMN IF EXISTS deletion_reason;
ALTER TABLE auth.users DROP COLUMN IF EXISTS deleted_at;
DROP INDEX IF EXISTS auth.idx_users_deleted_at;
