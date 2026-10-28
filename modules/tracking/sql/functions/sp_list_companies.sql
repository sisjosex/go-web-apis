CREATE OR REPLACE FUNCTION tracking.sp_list_companies(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_status character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, name character varying, email character varying, phone character varying, address text, city character varying, country character varying, registration_number character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone, vehicles_count bigint, drivers_count bigint, active_routes_count bigint, documents_due bigint, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    WITH pg AS (
        SELECT tc.id, CAST(COUNT(*) OVER () AS BIGINT) AS total_count
        FROM tracking.transport_companies tc
        WHERE tc.tenant_id = p_tenant_id
          -- TRACK-077: one company's row, so its page reads the same figures as the list.
          AND (p_id IS NULL OR tc.id = p_id)
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
        OFFSET (p_page - 1) * p_page_size
    )
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
        -- TRACK-074: the fleet figures, read for the page's rows only.
        (SELECT COUNT(*) FROM tracking.vehicles v WHERE v.company_id = tc.id),
        (SELECT COUNT(*) FROM tracking.drivers d WHERE d.tenant_id = p_tenant_id AND d.company_id = tc.id),
        (SELECT COUNT(*) FROM tracking.routes r WHERE r.company_id = tc.id AND r.is_active),
        -- The documents of its vehicles and drivers expired or within their type's warning.
        (SELECT COUNT(*) FROM (
             SELECT CAST('vehicle' AS VARCHAR) AS subject_type, v.id FROM tracking.vehicles v WHERE v.company_id = tc.id
             UNION ALL
             SELECT CAST('driver' AS VARCHAR), d.id FROM tracking.drivers d WHERE d.tenant_id = p_tenant_id AND d.company_id = tc.id
         ) sub
         INNER JOIN tracking.compliance_documents doc
             ON doc.tenant_id = p_tenant_id AND doc.subject_type = sub.subject_type AND doc.subject_id = sub.id
         INNER JOIN tracking.document_types dt ON dt.id = doc.document_type_id
         WHERE dt.is_active AND doc.expires_on IS NOT NULL AND doc.expires_on <= CURRENT_DATE + dt.warn_days_before),
        pg.total_count
    FROM pg
    INNER JOIN tracking.transport_companies tc ON tc.id = pg.id
    ORDER BY tc.name ASC, tc.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_companies(uuid,character varying,character varying,integer,integer,uuid) IS 'One page of a tenant''s transport companies by name, optionally narrowed by a name/registration-number search and a status, or to one company (p_id, TRACK-077); every row carries its vehicles, drivers, active routes and documents due over its fleet (TRACK-074) and the total_count the filters match';
