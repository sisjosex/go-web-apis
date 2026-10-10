CREATE OR REPLACE FUNCTION users.sp_update_user(p_tenant_id uuid, p_id uuid, p_first_name character varying DEFAULT NULL::character varying, p_last_name character varying DEFAULT NULL::character varying, p_phone character varying DEFAULT NULL::character varying, p_birthday date DEFAULT NULL::date, p_email character varying DEFAULT NULL::character varying, p_current_password character varying DEFAULT NULL::character varying, p_new_password character varying DEFAULT NULL::character varying, p_is_active boolean DEFAULT NULL::boolean, p_expiration_date timestamp with time zone DEFAULT NULL::timestamp with time zone, p_profile_picture_url text DEFAULT NULL::text, p_bio text DEFAULT NULL::text, p_website_url character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, first_name character varying, last_name character varying, phone character varying, birthday date, email character varying, profile_picture_url text, bio text, website_url character varying, is_active boolean, expiration_date date)
 LANGUAGE plpgsql
AS $function$
DECLARE
    lower_email      VARCHAR;
    stored_password  VARCHAR(255);
BEGIN
    -- Scope first: nothing below may reveal anything about a user of another workspace.
    PERFORM users.private_assert_tenant_member(p_tenant_id, p_id);

    lower_email           := auth.private_validate_email(p_email, FALSE);
    p_first_name          := TRIM(p_first_name);
    p_last_name           := TRIM(p_last_name);
    p_phone               := TRIM(p_phone);
    p_email               := TRIM(p_email);
    p_current_password    := NULLIF(TRIM(p_current_password), '');
    p_new_password        := NULLIF(TRIM(p_new_password), '');
    p_profile_picture_url := TRIM(p_profile_picture_url);
    p_bio                 := TRIM(p_bio);
    p_website_url         := TRIM(p_website_url);

    -- The login belongs to the account, not the workspace: only its sole workspace may change it.
    IF (p_new_password IS NOT NULL
        OR (lower_email IS NOT NULL
            AND lower_email IS DISTINCT FROM (SELECT LOWER(u.email) FROM auth.users u WHERE u.id = p_id)))
       AND to_regclass('tenancy.tenant_users') IS NOT NULL
       AND (SELECT COUNT(*) FROM tenancy.tenant_users tu WHERE tu.user_id = p_id AND tu.is_active = TRUE) > 1
    THEN
        RAISE EXCEPTION 'user.update.shared-account'
            USING ERRCODE = 'U0007',
                  DETAIL = 'The account belongs to more than one workspace; its email and password cannot be changed here';
    END IF;

    PERFORM auth.private_validate_user(p_id, lower_email);

    IF p_new_password <> '' THEN
        SELECT password INTO stored_password FROM auth.users WHERE users.id = p_id;

        IF p_current_password IS NULL OR crypt(p_current_password, stored_password) <> stored_password THEN
            RAISE EXCEPTION 'user.update.password.current-invalid'
                USING ERRCODE = 'U0001', DETAIL = 'Password does not match.';
        END IF;

        IF crypt(p_new_password, stored_password) = stored_password THEN
            RAISE EXCEPTION 'user.update.password.same-password'
                USING ERRCODE = 'U0002', DETAIL = 'Password cannot be the same.';
        END IF;

        p_new_password := auth.private_encrypt_password(p_new_password);
    END IF;

    UPDATE auth.users
    SET
        first_name      = COALESCE(p_first_name,      users.first_name),
        last_name       = COALESCE(p_last_name,       users.last_name),
        phone           = COALESCE(p_phone,           users.phone),
        birthday        = COALESCE(p_birthday,        users.birthday),
        email           = COALESCE(lower_email,       users.email),
        is_active       = COALESCE(p_is_active,       users.is_active),
        expiration_date = COALESCE(p_expiration_date, users.expiration_date),
        password        = COALESCE(p_new_password,    users.password),
        updated_at      = CURRENT_TIMESTAMP
    WHERE users.id = p_id;

    PERFORM auth.private_update_user_profile(p_id, p_profile_picture_url, p_bio, p_website_url);

    RETURN QUERY
    SELECT
        u.id::UUID,
        u.first_name::VARCHAR,
        u.last_name::VARCHAR,
        u.phone::VARCHAR,
        u.birthday::DATE,
        u.email::VARCHAR,
        p.profile_picture_url::TEXT,
        p.bio::TEXT,
        p.website_url::VARCHAR,
        u.is_active::BOOLEAN,
        u.expiration_date::DATE
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = p_id
    LIMIT 1;
END;
$function$;

COMMENT ON FUNCTION users.sp_update_user(uuid,uuid,character varying,character varying,character varying,date,character varying,character varying,character varying,boolean,timestamp with time zone,text,text,character varying) IS 'Update a user of the tenant and return the updated row. Raises user.not-found outside the tenant and user.update.shared-account for an email or password change on an account in more than one tenant.';
