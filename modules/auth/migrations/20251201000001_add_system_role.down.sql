-- Remove subscription_plan column
ALTER TABLE auth.users DROP COLUMN IF EXISTS subscription_plan;

-- Remove system_role column
ALTER TABLE auth.users DROP COLUMN IF EXISTS system_role;
