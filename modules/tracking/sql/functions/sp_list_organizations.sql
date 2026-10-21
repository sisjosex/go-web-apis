CREATE OR REPLACE FUNCTION tracking.sp_list_organizations(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_kind character varying DEFAULT NULL::character varying, p_is_active boolean DEFAULT NULL::boolean, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, kind character varying, name character varying, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, absence_cutoff_min integer, latitude numeric, longitude numeric, address character varying, stop_place_id uuid, empty_trip_cancel_min integer, delay_notice_min integer, my_role character varying, my_groups text[], total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT fr.*, CAST(me.role AS VARCHAR), me.groups, CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.organizations o
    CROSS JOIN LATERAL tracking.fn_organization_row(p_tenant_id, o.id) fr
    -- TRACK-067 D1: an organization user lists only its own organizations, each with its role there;
    -- staff (p_scope_user_id NULL) join nothing and read every row with my_role NULL.
    LEFT JOIN tracking.organization_members me ON me.organization_id = o.id AND me.user_id = p_scope_user_id
    WHERE o.tenant_id = p_tenant_id
      AND (p_scope_user_id IS NULL OR me.user_id IS NOT NULL)
      AND (p_search IS NULL OR p_search = '' OR o.name ILIKE '%' || p_search || '%')
      AND (p_kind IS NULL OR p_kind = '' OR o.kind = p_kind)
      AND (p_is_active IS NULL OR o.is_active = p_is_active)
    -- Alphabetical, so the page boundary follows the order the reader sees; o.id breaks ties.
    ORDER BY o.name ASC, o.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_organizations(uuid,character varying,character varying,boolean,integer,integer,uuid) IS 'One page of a tenant''s organizations by name, with their location (TRACK-038 D2) and stop (TRACK-045 D3), optionally narrowed by a name search, kind and status; an organization user (p_scope_user_id) sees only its own, with my_role and my_groups (TRACK-067); every row carries the total_count the filters match';
