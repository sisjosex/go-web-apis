CREATE OR REPLACE FUNCTION users.sp_get_user_by_id(p_tenant_id uuid, p_user_id uuid)
 RETURNS TABLE(id uuid, first_name character varying, last_name character varying, phone character varying, birthday date, email character varying, profile_picture_url text, bio text, website_url character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    PERFORM users.private_assert_tenant_member(p_tenant_id, p_user_id);

    RETURN QUERY
    SELECT
        u.id,
        u.first_name,
        u.last_name,
        u.phone,
        u.birthday,
        u.email,
        p.profile_picture_url,
        p.bio,
        p.website_url
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE u.id = p_user_id
    AND u.deleted_at IS NULL
    LIMIT 1;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'user.not-found'
            USING ERRCODE = 'U0002',
                  DETAIL = 'User not found or has been deleted';
    END IF;

EXCEPTION
    WHEN OTHERS THEN
        IF SQLSTATE ~ '^U0' THEN
            RAISE;
        ELSE
            RAISE EXCEPTION 'user.get.failed'
                USING ERRCODE = 'U0003',
                      DETAIL = SQLERRM;
        END IF;
END;
$function$;

COMMENT ON FUNCTION users.sp_get_user_by_id(uuid,uuid) IS 'Get a user of the tenant by ID with profile data. Raises user.not-found if the user is not an active member there, or is deleted.';
