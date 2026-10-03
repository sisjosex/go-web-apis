DROP FUNCTION auth.sp_set_user_locale(UUID, VARCHAR);
ALTER TABLE auth.users DROP CONSTRAINT chk_users_locale, DROP COLUMN locale;
