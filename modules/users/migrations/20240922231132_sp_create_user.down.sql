DROP FUNCTION IF EXISTS auth.private_create_user_profile;
DROP FUNCTION IF EXISTS auth.private_find_or_create_user;
DROP FUNCTION IF EXISTS auth.private_encrypt_password;
DROP FUNCTION IF EXISTS auth.private_validate_email;
DROP FUNCTION IF EXISTS users.sp_create_user;
DROP FUNCTION IF EXISTS users.sp_create_user_external;

-- Drop schema if empty (last migration)
-- DROP SCHEMA IF EXISTS users CASCADE;