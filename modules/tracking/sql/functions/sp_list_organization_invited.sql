CREATE OR REPLACE FUNCTION tracking.sp_list_organization_invited(p_tenant_id uuid, p_organization_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_user_ids uuid[] DEFAULT NULL::uuid[], p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(user_id uuid, first_name character varying, last_name character varying, phone character varying, email character varying, riders text[], total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) OR p_scope_user_id IS NOT NULL AND NOT (p_organization_id = ANY(tracking.fn_organization_scope(p_tenant_id, p_scope_user_id))) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT g.user_id, g.first_name, g.last_name, g.phone, g.email, g.riders, CAST(COUNT(*) OVER () AS BIGINT)
    FROM (
        SELECT rc.user_id,
               -- auth.users sits beside us on a shared database; elsewhere the contact's own copy answers.
               CAST(COALESCE(MAX(u.first_name), MAX(rc.name)) AS VARCHAR) AS first_name,
               CAST(MAX(u.last_name) AS VARCHAR) AS last_name,
               CAST(COALESCE(MAX(rc.phone), MAX(u.phone)) AS VARCHAR) AS phone,
               CAST(COALESCE(MAX(u.email), MAX(rc.email)) AS VARCHAR) AS email,
               ARRAY_AGG(DISTINCT CAST(r.first_name || ' ' || r.last_name AS TEXT)) AS riders
        FROM tracking.rider_contacts rc
        INNER JOIN tracking.riders r ON r.id = rc.rider_id
        LEFT JOIN auth.users u ON u.id = rc.user_id AND u.deleted_at IS NULL
        WHERE r.organization_id = p_organization_id
          AND rc.relation = 'guardian'
          AND rc.user_id IS NOT NULL
          AND (p_user_ids IS NULL OR rc.user_id = ANY(p_user_ids))
        GROUP BY rc.user_id
    ) g
    ORDER BY g.last_name NULLS LAST, g.first_name, g.user_id
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_organization_invited(uuid,uuid,uuid,uuid[],integer,integer) IS 'One page of the guardian accounts of an organization''s riders, each with the names of its riders, optionally only the given accounts (the invitations of one status, read from the main database first); an organization user only one of its own (TRACK-062)';
