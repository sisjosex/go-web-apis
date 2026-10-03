-- APP-009 D2: the language each person reads the app in, so an email written for them — a reset, a
-- digest a background job sends with no request behind it — goes out in it. The deployment serves
-- Bolivia, so a person who never chose one reads Spanish.

ALTER TABLE auth.users
    ADD COLUMN locale VARCHAR(2) NOT NULL DEFAULT 'es',
    ADD CONSTRAINT chk_users_locale CHECK (locale IN ('es', 'en'));

COMMENT ON COLUMN auth.users.locale IS
'The language this person reads the app and its emails in; set when they switch it (APP-009 D2)';

CREATE FUNCTION auth.sp_set_user_locale(
    p_user_id UUID,
    p_locale VARCHAR
)
RETURNS VOID AS $$
    UPDATE auth.users u
    SET locale = p_locale
    WHERE u.id = p_user_id AND u.locale IS DISTINCT FROM p_locale;
$$ LANGUAGE sql;

COMMENT ON FUNCTION auth.sp_set_user_locale(UUID, VARCHAR) IS
'Records the language a person switched the app to; a no-op when it is already that one (APP-009 D2)';
