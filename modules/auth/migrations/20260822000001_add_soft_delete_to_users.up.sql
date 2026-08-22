-- Soft delete columns for auth.users.
-- Lives in auth, not users: auth owns the table, and the tenancy module
-- backfill needs deleted_at while users itself migrates after tenancy.
ALTER TABLE auth.users 
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;

-- Add index for soft delete queries
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON auth.users (deleted_at);

-- Add optional deletion reason column
ALTER TABLE auth.users 
ADD COLUMN IF NOT EXISTS deletion_reason TEXT DEFAULT NULL;

COMMENT ON COLUMN auth.users.deleted_at IS 'Soft delete timestamp. NULL = active user, NOT NULL = deleted user';
COMMENT ON COLUMN auth.users.deletion_reason IS 'Optional reason for user deletion';
