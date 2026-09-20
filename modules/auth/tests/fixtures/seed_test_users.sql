-- ================================================================
-- TEST DATA SEED FILE - Easy to Edit and Adjust
-- ================================================================
-- This file seeds initial test users into the database
-- Uses stored procedures to maintain consistency with production logic
--
-- To generate new password hashes, use PostgreSQL:
--   SELECT crypt('YourPassword', gen_salt('md5'));
--
-- Then copy the hash into the password field below

-- ================================================================
-- SUPER ADMIN USER
-- ================================================================
-- Email: superadmin@test.local
-- Password: SuperAdmin123!
-- Hash: $1$zNmsaqWz$mYK/yYZAhVxonuGwrJfjJ1
-- Role: super_admin
-- ================================================================
DELETE FROM auth.users WHERE email = 'superadmin@test.local';
SELECT * FROM auth.sp_register_user(
	p_email := 'superadmin@test.local',
	p_first_name := 'Super',
	p_last_name := 'Admin',
	p_password := 'SuperAdmin123!'
);

-- Update role and plan (fixtures use admin roles)
UPDATE auth.users 
SET 
	system_role = 'super_admin',
	subscription_plan = 'enterprise',
	email_verified = true
WHERE email = 'superadmin@test.local';

-- ================================================================
-- ADMIN USER
-- ================================================================
-- Email: admin@test.local
-- Password: Admin123!
-- Hash: $1$8PTpwORg$eGb/Hck0ecfIegC60cCAE0
-- Role: admin
-- ================================================================
DELETE FROM auth.users WHERE email = 'admin@test.local';
SELECT * FROM auth.sp_register_user(
	p_email := 'admin@test.local',
	p_first_name := 'Admin',
	p_last_name := 'User',
	p_password := 'Admin123!'
);

-- Update role and plan (fixtures use admin roles)
UPDATE auth.users 
SET 
	system_role = 'admin',
	subscription_plan = 'pro',
	email_verified = true
WHERE email = 'admin@test.local';

-- ================================================================
-- ORGANIZATION USER (TRACK-015)
-- ================================================================
-- Email: orguser@test.local
-- Password: OrgUser123!
-- Tenant access level: organization — sees only the organizations it is a member of
-- ================================================================
DELETE FROM auth.users WHERE email = 'orguser@test.local';
SELECT * FROM auth.sp_register_user(
	p_email := 'orguser@test.local',
	p_first_name := 'Org',
	p_last_name := 'User',
	p_password := 'OrgUser123!'
);

UPDATE auth.users
SET
	system_role = 'user',
	subscription_plan = 'free',
	email_verified = true
WHERE email = 'orguser@test.local';

-- ================================================================
-- PORTAL USER (TRACK-015)
-- ================================================================
-- Email: portal@test.local
-- Password: Portal123!
-- Tenant access level: portal — mobile only, refused on every web tenant route
-- ================================================================
DELETE FROM auth.users WHERE email = 'portal@test.local';
SELECT * FROM auth.sp_register_user(
	p_email := 'portal@test.local',
	p_first_name := 'Portal',
	p_last_name := 'Guardian',
	p_password := 'Portal123!'
);

UPDATE auth.users
SET
	system_role = 'user',
	subscription_plan = 'free',
	email_verified = true
WHERE email = 'portal@test.local';

-- ================================================================
-- ADD MORE TEST USERS BELOW
-- ================================================================
-- Example format (uncomment to use):
--
-- DELETE FROM auth.users WHERE email = 'testuser@test.local';
-- SELECT * FROM auth.sp_register_user(
-- 	p_email := 'testuser@test.local',
-- 	p_first_name := 'Test',
-- 	p_last_name := 'User',
-- 	p_password := 'TestPass123!'
-- );
--
-- UPDATE auth.users 
-- SET 
-- 	system_role = 'user',
-- 	subscription_plan = 'free',
-- 	email_verified = true
-- WHERE email = 'testuser@test.local';
