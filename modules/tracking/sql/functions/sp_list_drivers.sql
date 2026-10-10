CREATE OR REPLACE FUNCTION tracking.sp_list_drivers(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_company_id uuid DEFAULT NULL::uuid, p_status character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, tenant_id uuid, company_id uuid, company_name character varying, user_id uuid, user_email character varying, first_name character varying, last_name character varying, phone character varying, license_number character varying, license_class character varying, license_expires_on date, status character varying, service_blocked boolean, created_at timestamp without time zone, updated_at timestamp without time zone, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        d.company_id,
        CAST(tc.name AS VARCHAR),
        d.user_id,
        CAST(u.email AS VARCHAR),
        CAST(d.first_name AS VARCHAR),
        CAST(d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(d.license_number AS VARCHAR),
        CAST(d.license_class AS VARCHAR),
        d.license_expires_on,
        CAST(d.status AS VARCHAR),
        tracking.fn_service_blocked(d.tenant_id, 'driver', d.id),
        d.created_at,
        d.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.drivers d
    INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
    LEFT JOIN auth.users u ON u.id = d.user_id AND u.deleted_at IS NULL
    WHERE d.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR d.company_id = p_company_id)
      AND (p_status IS NULL OR p_status = '' OR d.status = p_status)
      AND (p_search IS NULL OR p_search = ''
           OR d.first_name ILIKE '%' || p_search || '%'
           OR d.last_name ILIKE '%' || p_search || '%'
           OR d.license_number ILIKE '%' || p_search || '%')
    ORDER BY d.last_name ASC, d.first_name ASC, d.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_drivers(uuid,character varying,uuid,character varying,integer,integer) IS 'One page of a tenant''s drivers by surname, optionally narrowed by a name or licence search, carrier and status; every row carries its carrier, its linked account''s email, whether an expired blocking document stands against it (TRACK-016 D2) and the total_count the filters match';
