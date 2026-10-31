CREATE OR REPLACE FUNCTION users.sp_list_users(p_tenant_id uuid, p_page integer DEFAULT 1, p_limit integer DEFAULT 10, p_search text DEFAULT NULL::text, p_status text DEFAULT NULL::text, p_sort text DEFAULT 'created_at'::text, p_order text DEFAULT 'desc'::text, p_exclude_user_id uuid DEFAULT NULL::uuid, p_level text DEFAULT NULL::text, p_role_id uuid DEFAULT NULL::uuid, p_pending boolean DEFAULT false)
 RETURNS TABLE(id uuid, first_name character varying, last_name character varying, phone character varying, birthday date, email character varying, profile_picture_url text, bio text, website_url character varying, is_active boolean, created_at timestamp with time zone, expiration_date date, total_count bigint, tenant_role character varying, role_id uuid, role_name character varying, role_count integer, modules text[], last_sign_in_at timestamp with time zone, invitation_pending boolean)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_offset      INT;
    v_total_count BIGINT;
    v_enabled     TEXT[];
BEGIN
    v_offset := (p_page - 1) * p_limit;

    SELECT COALESCE(array_agg(tm.module_code::TEXT ORDER BY tm.module_code), ARRAY[]::TEXT[])
    INTO v_enabled
    FROM tenancy.tenant_modules tm
    WHERE tm.tenant_id = p_tenant_id AND tm.is_enabled;

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
        AND ((p_level IS NULL AND tu.role NOT IN ('driver', 'portal'))
             OR (p_level = 'web' AND tu.role IN ('owner', 'admin', 'member', 'viewer'))
             OR (p_level = 'admin' AND tu.role IN ('owner', 'admin'))
             OR tu.role = p_level)
        AND (p_role_id IS NULL OR EXISTS (
            SELECT 1 FROM tenancy.user_roles ur
            WHERE ur.user_id = u.id AND ur.tenant_id = p_tenant_id AND ur.role_id = p_role_id))
        AND (NOT p_pending OR NOT EXISTS (SELECT 1 FROM auth.user_sessions us WHERE us.user_id = u.id))
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
        tu.role::VARCHAR,
        r.first_role_id,
        r.first_role_name::VARCHAR,
        COALESCE(r.roles, 0)::INT,
        CASE
            WHEN tu.role IN ('owner', 'admin') THEN v_enabled
            WHEN tu.role = 'organization' THEN ARRAY(SELECT e FROM unnest(v_enabled) e WHERE e = 'tracking')
            ELSE ARRAY(
                SELECT DISTINCT split_part(rp.permission_code, ':', 1)
                FROM tenancy.user_roles ur
                INNER JOIN tenancy.role_permissions rp ON rp.role_id = ur.role_id
                WHERE ur.user_id = u.id AND ur.tenant_id = p_tenant_id
                  AND split_part(rp.permission_code, ':', 1) = ANY (v_enabled)
                ORDER BY 1)
        END,
        s.last_login,
        s.last_login IS NULL
    FROM auth.users u
    INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    LEFT JOIN LATERAL (
        SELECT
            (array_agg(ro.id ORDER BY ur.assigned_at, ro.id))[1] AS first_role_id,
            (array_agg(ro.name ORDER BY ur.assigned_at, ro.id))[1] AS first_role_name,
            COUNT(*) AS roles
        FROM tenancy.user_roles ur
        INNER JOIN tenancy.roles ro ON ro.id = ur.role_id
        WHERE ur.user_id = u.id AND ur.tenant_id = p_tenant_id
    ) r ON TRUE
    LEFT JOIN LATERAL (
        SELECT MAX(us.login_time) AS last_login
        FROM auth.user_sessions us
        WHERE us.user_id = u.id
    ) s ON TRUE
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
        AND ((p_level IS NULL AND tu.role NOT IN ('driver', 'portal'))
             OR (p_level = 'web' AND tu.role IN ('owner', 'admin', 'member', 'viewer'))
             OR (p_level = 'admin' AND tu.role IN ('owner', 'admin'))
             OR tu.role = p_level)
        AND (p_role_id IS NULL OR EXISTS (
            SELECT 1 FROM tenancy.user_roles ur
            WHERE ur.user_id = u.id AND ur.tenant_id = p_tenant_id AND ur.role_id = p_role_id))
        AND (NOT p_pending OR NOT EXISTS (SELECT 1 FROM auth.user_sessions us WHERE us.user_id = u.id))
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

COMMENT ON FUNCTION users.sp_list_users(uuid,integer,integer,text,text,text,text,uuid,text,uuid,boolean) IS 'List users scoped to a specific tenant with pagination, filters, sorting, and tenant role, each with its first role, the modules its role reaches, its last sign-in and whether it never signed in; p_level narrows to portal, driver, organization, admin (owner, admin) or web (owner, admin, member, viewer), and with none the app-only portal and driver accounts stay out (USERS-015).';
