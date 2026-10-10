CREATE OR REPLACE FUNCTION tracking.sp_list_companies(p_registration_number character varying, p_status character varying)
 RETURNS TABLE(out_id uuid, out_name character varying, out_email character varying, out_phone character varying, out_address text, out_city character varying, out_country character varying, out_registration_number character varying, out_status character varying, out_created_at timestamp without time zone, out_updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT 
        id,
        name,
        email,
        phone,
        address,
        city,
        country,
        registration_number,
        status,
        created_at,
        updated_at
    FROM tracking.transport_companies
    WHERE 
        (p_registration_number IS NULL OR registration_number = p_registration_number)
        AND (p_status IS NULL OR status = p_status)
    ORDER BY created_at DESC;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_list_companies(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_status character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, tenant_id uuid, name character varying, email character varying, phone character varying, address text, city character varying, country character varying, registration_number character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        tc.id,
        tc.tenant_id,
        CAST(tc.name AS VARCHAR),
        CAST(tc.email AS VARCHAR),
        CAST(tc.phone AS VARCHAR),
        tc.address,
        CAST(tc.city AS VARCHAR),
        CAST(tc.country AS VARCHAR),
        CAST(tc.registration_number AS VARCHAR),
        CAST(tc.status AS VARCHAR),
        tc.created_at,
        tc.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.transport_companies tc
    WHERE tc.tenant_id = p_tenant_id
      AND (
          p_search IS NULL
          OR p_search = ''
          OR tc.name                ILIKE '%' || p_search || '%'
          OR tc.registration_number ILIKE '%' || p_search || '%'
      )
      AND (p_status IS NULL OR p_status = '' OR tc.status = p_status)
    -- The list is read alphabetically, so the page is cut that way too: ordering by creation
    -- date and sorting the page in the browser would only sort the twenty rows on screen.
    -- tc.id breaks ties, so two companies of the same name never swap between pages.
    ORDER BY tc.name ASC, tc.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_companies(uuid,character varying,character varying,integer,integer) IS 'One page of a tenant''s transport companies by name, optionally narrowed by a name/registration-number search and a status; every row carries the total_count the filters match';
