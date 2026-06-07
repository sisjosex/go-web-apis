-- Restore the original sp_update_user signature (without is_active/expiration_date in return)
DROP FUNCTION IF EXISTS users.sp_update_user(UUID, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, TIMESTAMPTZ, TEXT, TEXT, VARCHAR);
