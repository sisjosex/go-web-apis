-- Rollback: sp_register_user
-- Module: auth

DROP FUNCTION IF EXISTS auth.sp_register_user(VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, TEXT, TEXT, VARCHAR);
DROP FUNCTION IF EXISTS auth.sp_register_user_external(VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, TEXT, TEXT, VARCHAR);

