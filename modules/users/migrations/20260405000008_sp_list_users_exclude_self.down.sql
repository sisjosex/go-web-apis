-- Removes the p_exclude_user_id parameter from sp_list_users.
DROP FUNCTION IF EXISTS users.sp_list_users(INT, INT, TEXT, TEXT, TEXT, TEXT, UUID);

CREATE FUNCTION users.sp_list_users(
    p_page   INT  DEFAULT 1,
    p_limit  INT  DEFAULT 10,
    p_search TEXT DEFAULT NULL,
    p_status TEXT DEFAULT NULL,
    p_sort   TEXT DEFAULT 'created_at',
    p_order  TEXT DEFAULT 'desc'
)
RETURNS TABLE (
    id                  UUID,
    first_name          VARCHAR,
    last_name           VARCHAR,
    phone               VARCHAR,
    birthday            DATE,
    email               VARCHAR,
    profile_picture_url TEXT,
    bio                 TEXT,
    website_url         VARCHAR,
    is_active           BOOLEAN,
    created_at          TIMESTAMPTZ,
    expiration_date     DATE,
    total_count         BIGINT
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_offset      INT;
    v_total_count BIGINT;
BEGIN
    v_offset := (p_page - 1) * p_limit;

    SELECT COUNT(*) INTO v_total_count
    FROM auth.users u
    WHERE
        (p_search IS NULL OR (
            LOWER(u.email)      LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.first_name) LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.last_name)  LIKE LOWER('%' || p_search || '%')
        ))
        AND (p_status IS NULL OR (
            CASE
                WHEN p_status = 'active'   THEN u.is_active = TRUE  AND (u.expiration_date IS NULL OR u.expiration_date > CURRENT_DATE)
                WHEN p_status = 'inactive' THEN u.is_active = FALSE
                WHEN p_status = 'expired'  THEN u.expiration_date IS NOT NULL AND u.expiration_date <= CURRENT_DATE
                ELSE TRUE
            END
        ))
        AND u.deleted_at IS NULL;

    RETURN QUERY
    SELECT
        u.id::UUID, u.first_name::VARCHAR, u.last_name::VARCHAR, u.phone::VARCHAR,
        u.birthday::DATE, u.email::VARCHAR, p.profile_picture_url::TEXT, p.bio::TEXT,
        p.website_url::VARCHAR, u.is_active::BOOLEAN, u.created_at::TIMESTAMPTZ,
        u.expiration_date::DATE, v_total_count::BIGINT
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE
        (p_search IS NULL OR (
            LOWER(u.email)      LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.first_name) LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.last_name)  LIKE LOWER('%' || p_search || '%')
        ))
        AND (p_status IS NULL OR (
            CASE
                WHEN p_status = 'active'   THEN u.is_active = TRUE  AND (u.expiration_date IS NULL OR u.expiration_date > CURRENT_DATE)
                WHEN p_status = 'inactive' THEN u.is_active = FALSE
                WHEN p_status = 'expired'  THEN u.expiration_date IS NOT NULL AND u.expiration_date <= CURRENT_DATE
                ELSE TRUE
            END
        ))
        AND u.deleted_at IS NULL
    ORDER BY
        CASE WHEN p_sort = 'email'      AND p_order = 'asc'  THEN u.email      END ASC,
        CASE WHEN p_sort = 'email'      AND p_order = 'desc' THEN u.email      END DESC,
        CASE WHEN p_sort = 'first_name' AND p_order = 'asc'  THEN u.first_name END ASC,
        CASE WHEN p_sort = 'first_name' AND p_order = 'desc' THEN u.first_name END DESC,
        CASE WHEN p_sort = 'last_name'  AND p_order = 'asc'  THEN u.last_name  END ASC,
        CASE WHEN p_sort = 'last_name'  AND p_order = 'desc' THEN u.last_name  END DESC,
        CASE WHEN p_sort = 'created_at' AND p_order = 'asc'  THEN u.created_at END ASC,
        CASE WHEN p_sort = 'created_at' AND p_order = 'desc' THEN u.created_at END DESC
    LIMIT p_limit OFFSET v_offset;

EXCEPTION
    WHEN OTHERS THEN
        RAISE EXCEPTION 'user.list.failed' USING ERRCODE = 'U0001', DETAIL = SQLERRM;
END;
$$;
