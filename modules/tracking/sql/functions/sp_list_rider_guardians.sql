CREATE OR REPLACE FUNCTION tracking.sp_list_rider_guardians(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid)
 RETURNS TABLE(user_id uuid, email character varying, first_name character varying, last_name character varying, phone character varying, is_primary boolean)
 LANGUAGE plpgsql
AS $function$
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id);

    RETURN QUERY
    SELECT g.user_id, g.email, g.first_name, g.last_name, g.phone, g.is_primary FROM (
        SELECT DISTINCT ON (COALESCE(rc.user_id, rc.id))
            rc.user_id,
            CAST(COALESCE(u.email, rc.email) AS VARCHAR) AS email,
            CAST(COALESCE(u.first_name, rc.name) AS VARCHAR) AS first_name,
            CAST(u.last_name AS VARCHAR) AS last_name,
            CAST(COALESCE(rc.phone, u.phone) AS VARCHAR) AS phone,
            rc.is_primary
        FROM tracking.rider_contacts rc
        -- auth.users sits beside us on a shared database; elsewhere the contact's own copy answers.
        LEFT JOIN auth.users u ON u.id = rc.user_id AND u.deleted_at IS NULL
        WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian'
          AND (rc.user_id IS NOT NULL OR (rc.name IS NOT NULL AND rc.phone IS NOT NULL))
        ORDER BY COALESCE(rc.user_id, rc.id), rc.is_primary DESC
    ) g
    ORDER BY (g.user_id IS NULL), g.is_primary DESC, g.first_name, g.last_name;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_rider_guardians(uuid,uuid,uuid) IS 'The guardians of a rider: accounts first, then contacts without one (user_id NULL, TRACK-037 D3); raises tracking.rider.not-found outside the tenant or the caller''s organization scope';
