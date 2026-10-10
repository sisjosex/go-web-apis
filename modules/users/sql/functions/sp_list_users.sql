CREATE OR REPLACE FUNCTION users.sp_list_users(p_tenant_id uuid, p_page integer DEFAULT 1, p_limit integer DEFAULT 10, p_search text DEFAULT NULL::text, p_status text DEFAULT NULL::text, p_sort text DEFAULT 'created_at'::text, p_order text DEFAULT 'desc'::text, p_exclude_user_id uuid DEFAULT NULL::uuid, p_level text DEFAULT NULL::text)
 RETURNS TABLE(id uuid, first_name character varying, last_name character varying, phone character varying, birthday date, email character varying, profile_picture_url text, bio text, website_url character varying, is_active boolean, created_at timestamp with time zone, expiration_date date, total_count bigint, tenant_role character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_offset      INT;
    v_total_count BIGINT;
BEGIN
    v_offset := (p_page - 1) * p_limit;

    SELECT COUNT(*)
    INTO v_total_count
    FROM auth.users u
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
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
        AND (p_exclude_user_id IS NULL OR u.id != p_exclude_user_id)
        AND (p_level IS NULL
             OR (p_level = 'web' AND tu.role IN ('owner', 'admin', 'member', 'viewer'))
             OR tu.role = p_level)
        AND u.deleted_at IS NULL;

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
        u.created_at::TIMESTAMPTZ,
        u.expiration_date::DATE,
        v_total_count::BIGINT,
        tu.role::VARCHAR
    FROM auth.users u
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
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
        AND (p_exclude_user_id IS NULL OR u.id != p_exclude_user_id)
        AND (p_level IS NULL
             OR (p_level = 'web' AND tu.role IN ('owner', 'admin', 'member', 'viewer'))
             OR tu.role = p_level)
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
    LIMIT  p_limit
    OFFSET v_offset;

EXCEPTION
    WHEN OTHERS THEN
        RAISE EXCEPTION 'user.list.failed'
            USING ERRCODE = 'U0001',
                  DETAIL  = SQLERRM;
END;
$function$;

COMMENT ON FUNCTION users.sp_list_users(uuid,integer,integer,text,text,text,text,uuid,text) IS 'List users scoped to a specific tenant with pagination, filters, sorting, and tenant role; p_level narrows to portal, driver, organization or web (owner, admin, member, viewer).';
