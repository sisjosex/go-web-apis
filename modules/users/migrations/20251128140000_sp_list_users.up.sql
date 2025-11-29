/*
Stored Procedure: sp_list_users
Description: List users with pagination, search, and filters (optimized with indexed columns)
Parameters:
  - p_page: Page number (default 1)
  - p_limit: Items per page (default 10)
  - p_search: Search text in email, first_name, last_name (optional)
  - p_status: Filter by is_active status (optional: 'active', 'inactive', 'expired')
  - p_sort: Sort field (default 'created_at')
  - p_order: Sort order (default 'desc')
Returns: TABLE of users + total count

Usage:
-- List all users (page 1, 10 items)
SELECT * FROM auth.sp_list_users(
  p_page := 1,
  p_limit := 10
);

-- Search users by name/email
SELECT * FROM auth.sp_list_users(
  p_page := 1,
  p_limit := 10,
  p_search := 'jose'
);

-- Filter active users, sort by email
SELECT * FROM auth.sp_list_users(
  p_page := 1,
  p_limit := 20,
  p_status := 'active',
  p_sort := 'email',
  p_order := 'asc'
);
*/

CREATE OR REPLACE FUNCTION auth.sp_list_users(
    p_page INT DEFAULT 1,
    p_limit INT DEFAULT 10,
    p_search TEXT DEFAULT NULL,
    p_status TEXT DEFAULT NULL,
    p_sort TEXT DEFAULT 'created_at',
    p_order TEXT DEFAULT 'desc'
)
RETURNS TABLE (
    id UUID,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    birthday DATE,
    email VARCHAR,
    profile_picture_url TEXT,
    bio TEXT,
    website_url VARCHAR,
    total_count BIGINT  -- Total count for pagination
) 
LANGUAGE plpgsql
AS $$
DECLARE
    v_offset INT;
    v_total_count BIGINT;
BEGIN
    -- Calculate offset
    v_offset := (p_page - 1) * p_limit;
    
    -- Get total count (for pagination metadata)
    SELECT COUNT(*)
    INTO v_total_count
    FROM auth.users u
    WHERE
        -- Search filter (case-insensitive)
        (p_search IS NULL OR (
            LOWER(u.email) LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.first_name) LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.last_name) LIKE LOWER('%' || p_search || '%')
        ))
        -- Status filter
        AND (p_status IS NULL OR (
            CASE 
                WHEN p_status = 'active' THEN u.is_active = TRUE AND (u.expiration_date IS NULL OR u.expiration_date > CURRENT_DATE)
                WHEN p_status = 'inactive' THEN u.is_active = FALSE
                WHEN p_status = 'expired' THEN u.expiration_date IS NOT NULL AND u.expiration_date <= CURRENT_DATE
                ELSE TRUE
            END
        ))
        -- Exclude soft-deleted users (if deleted_at column exists)
        AND u.deleted_at IS NULL;
    
    -- Return paginated results with total count
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
        p.website_url,
        v_total_count  -- Same count for all rows (client extracts from first row)
    FROM auth.users u
    LEFT JOIN auth.user_profile p ON u.id = p.user_id
    WHERE
        -- Same filters as count query
        (p_search IS NULL OR (
            LOWER(u.email) LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.first_name) LIKE LOWER('%' || p_search || '%') OR
            LOWER(u.last_name) LIKE LOWER('%' || p_search || '%')
        ))
        AND (p_status IS NULL OR (
            CASE 
                WHEN p_status = 'active' THEN u.is_active = TRUE AND (u.expiration_date IS NULL OR u.expiration_date > CURRENT_DATE)
                WHEN p_status = 'inactive' THEN u.is_active = FALSE
                WHEN p_status = 'expired' THEN u.expiration_date IS NOT NULL AND u.expiration_date <= CURRENT_DATE
                ELSE TRUE
            END
        ))
        AND u.deleted_at IS NULL
    ORDER BY
        CASE WHEN p_sort = 'email' AND p_order = 'asc' THEN u.email END ASC,
        CASE WHEN p_sort = 'email' AND p_order = 'desc' THEN u.email END DESC,
        CASE WHEN p_sort = 'first_name' AND p_order = 'asc' THEN u.first_name END ASC,
        CASE WHEN p_sort = 'first_name' AND p_order = 'desc' THEN u.first_name END DESC,
        CASE WHEN p_sort = 'last_name' AND p_order = 'asc' THEN u.last_name END ASC,
        CASE WHEN p_sort = 'last_name' AND p_order = 'desc' THEN u.last_name END DESC,
        CASE WHEN p_sort = 'created_at' AND p_order = 'asc' THEN u.created_at END ASC,
        CASE WHEN p_sort = 'created_at' AND p_order = 'desc' THEN u.created_at END DESC
    LIMIT p_limit
    OFFSET v_offset;
    
EXCEPTION
    WHEN OTHERS THEN
        RAISE EXCEPTION 'user.list.failed'
            USING ERRCODE = 'U0001',
                  DETAIL = SQLERRM;
END;
$$;

-- Add comment
COMMENT ON FUNCTION auth.sp_list_users(INT, INT, TEXT, TEXT, TEXT, TEXT) IS 
'List users with pagination, search (email/name), status filter, and sorting. Returns total count for pagination.';
