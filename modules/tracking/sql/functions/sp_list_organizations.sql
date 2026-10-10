CREATE OR REPLACE FUNCTION tracking.sp_list_organizations(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_kind character varying DEFAULT NULL::character varying, p_is_active boolean DEFAULT NULL::boolean, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, tenant_id uuid, kind character varying, name character varying, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, absence_cutoff_min integer, latitude numeric, longitude numeric, address character varying, stop_place_id uuid, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT fr.*, CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.organizations o
    CROSS JOIN LATERAL tracking.fn_organization_row(p_tenant_id, o.id) fr
    WHERE o.tenant_id = p_tenant_id
      AND (p_search IS NULL OR p_search = '' OR o.name ILIKE '%' || p_search || '%')
      AND (p_kind IS NULL OR p_kind = '' OR o.kind = p_kind)
      AND (p_is_active IS NULL OR o.is_active = p_is_active)
    -- Alphabetical, so the page boundary follows the order the reader sees; o.id breaks ties.
    ORDER BY o.name ASC, o.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_organizations(uuid,character varying,character varying,boolean,integer,integer) IS 'One page of a tenant''s organizations by name, with their location (TRACK-038 D2) and stop (TRACK-045 D3), optionally narrowed by a name search, kind and status; every row carries the total_count the filters match';
