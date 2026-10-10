CREATE OR REPLACE FUNCTION tracking.sp_list_calendars(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, tenant_id uuid, organization_id uuid, name character varying, dates_count integer, created_at timestamp without time zone, updated_at timestamp without time zone, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        c.id,
        c.tenant_id,
        c.organization_id,
        CAST(c.name AS VARCHAR),
        CAST((SELECT COUNT(*) FROM tracking.calendar_dates cd WHERE cd.calendar_id = c.id) AS INT),
        c.created_at,
        c.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.calendars c
    WHERE c.tenant_id = p_tenant_id
      AND (p_search IS NULL OR p_search = '' OR c.name ILIKE '%' || p_search || '%')
    ORDER BY c.name ASC, c.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_calendars(uuid,character varying,integer,integer) IS 'One page of a tenant''s calendars with how many dates each holds; every row carries the total_count the filters match';
