CREATE OR REPLACE FUNCTION tracking.sp_list_vehicles(p_company_id uuid, p_vehicle_type character varying, p_status character varying)
 RETURNS TABLE(id uuid, company_id uuid, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        v.plate_number,
        v.vehicle_type,
        v.brand,
        v.model,
        v.year,
        v.capacity,
        v.gps_device_id,
        v.status,
        v.created_at,
        v.updated_at
    FROM tracking.vehicles v
    WHERE
        (p_company_id IS NULL OR v.company_id = p_company_id)
        AND (p_vehicle_type IS NULL OR v.vehicle_type = p_vehicle_type)
        AND (p_status IS NULL OR v.status = p_status)
    ORDER BY v.created_at DESC;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_list_vehicles(p_tenant_id uuid, p_company_id uuid DEFAULT NULL::uuid, p_vehicle_type character varying DEFAULT NULL::character varying, p_status character varying DEFAULT NULL::character varying, p_search character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_view character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, company_id uuid, company_name character varying, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, service_blocked boolean, created_at timestamp without time zone, updated_at timestamp without time zone, default_driver_id uuid, driver_name character varying, today_route_name character varying, last_seen_at timestamp with time zone, next_document jsonb, counts jsonb, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    -- TRACK-075: the three view flags are what the chips count, so they are read for every vehicle
    -- the other filters keep (one indexed probe each); everything else only for the page.
    WITH f AS MATERIALIZED (
        SELECT
            v.id,
            v.plate_number,
            CAST(tc.name AS VARCHAR) AS company_name,
            lp.recorded_at AS last_seen_at,
            EXISTS (SELECT 1 FROM tracking.trips t WHERE t.vehicle_id = v.id AND t.status = 'in_progress') AS on_route,
            lp.vehicle_id IS NULL AS no_gps,
            EXISTS (
                SELECT 1 FROM tracking.compliance_documents d
                INNER JOIN tracking.document_types dt ON dt.id = d.document_type_id
                WHERE d.tenant_id = p_tenant_id AND d.subject_type = 'vehicle' AND d.subject_id = v.id
                  AND dt.is_active AND d.expires_on IS NOT NULL
                  AND d.expires_on <= CURRENT_DATE + dt.warn_days_before
            ) AS docs_due
        FROM tracking.vehicles v
        -- Not a LEFT JOIN: company_id is NOT NULL and this join is what scopes the rows to the
        -- tenant, so an outer join would only widen the result to other tenants' vehicles.
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = v.id
        WHERE tc.tenant_id = p_tenant_id
          AND (p_company_id IS NULL OR v.company_id = p_company_id)
          AND (p_vehicle_type IS NULL OR p_vehicle_type = '' OR v.vehicle_type = p_vehicle_type)
          AND (p_status IS NULL OR p_status = '' OR v.status = p_status)
          AND (
              p_search IS NULL
              OR p_search = ''
              OR v.plate_number ILIKE '%' || p_search || '%'
          )
    ),
    c AS (
        SELECT jsonb_build_object(
            'all',      COUNT(*),
            'on_route', COUNT(*) FILTER (WHERE f.on_route),
            'no_gps',   COUNT(*) FILTER (WHERE f.no_gps),
            'docs_due', COUNT(*) FILTER (WHERE f.docs_due)
        ) AS counts
        FROM f
    ),
    pg AS (
        SELECT f.id, f.company_name, f.last_seen_at, CAST(COUNT(*) OVER () AS BIGINT) AS total_count
        FROM f
        WHERE p_view IS NULL OR p_view = '' OR p_view = 'all'
           OR (p_view = 'on_route' AND f.on_route)
           OR (p_view = 'no_gps' AND f.no_gps)
           OR (p_view = 'docs_due' AND f.docs_due)
        ORDER BY f.plate_number ASC, f.id ASC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        v.id,
        v.company_id,
        pg.company_name,
        CAST(v.plate_number AS VARCHAR),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(v.brand AS VARCHAR),
        CAST(v.model AS VARCHAR),
        v.year,
        v.capacity,
        CAST(v.gps_device_id AS VARCHAR),
        CAST(v.status AS VARCHAR),
        CASE WHEN v.id IS NOT NULL THEN tracking.fn_service_blocked(p_tenant_id, 'vehicle', v.id) END,
        v.created_at,
        v.updated_at,
        v.default_driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(tr.route_name AS VARCHAR),
        pg.last_seen_at,
        nd.doc,
        c.counts,
        COALESCE(pg.total_count, 0)
    -- An empty page still answers one row, id NULL, so the chips keep their counts.
    FROM c
    LEFT JOIN pg ON true
    LEFT JOIN tracking.vehicles v ON v.id = pg.id
    LEFT JOIN tracking.drivers d ON d.id = v.default_driver_id
    -- The trip in progress, else the next one planned on its own local today.
    LEFT JOIN LATERAL (
        SELECT r.route_name FROM tracking.trips t
        INNER JOIN tracking.routes r ON r.id = t.route_id
        WHERE t.vehicle_id = v.id
          AND (t.status = 'in_progress'
               OR (t.status = 'planned' AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)))
        ORDER BY (t.status = 'in_progress') DESC, t.planned_start ASC
        LIMIT 1
    ) tr ON true
    -- The soonest document expired or within its type's warning; NULL while every one is valid.
    LEFT JOIN LATERAL (
        SELECT jsonb_build_object(
            'type_name',  dt.name,
            'expires_on', doc.expires_on,
            'status',     CASE WHEN doc.expires_on < CURRENT_DATE THEN 'expired' ELSE 'expiring' END
        ) AS doc
        FROM tracking.compliance_documents doc
        INNER JOIN tracking.document_types dt ON dt.id = doc.document_type_id
        WHERE doc.tenant_id = p_tenant_id AND doc.subject_type = 'vehicle' AND doc.subject_id = v.id
          AND dt.is_active AND doc.expires_on IS NOT NULL
          AND doc.expires_on <= CURRENT_DATE + dt.warn_days_before
        ORDER BY doc.expires_on ASC, doc.id ASC
        LIMIT 1
    ) nd ON true
    ORDER BY v.plate_number ASC, v.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_vehicles(uuid,uuid,character varying,character varying,character varying,integer,integer,character varying) IS 'One page of a tenant''s vehicles by plate number, optionally narrowed by company, type, status, a plate search and a view (on_route, no_gps, docs_due — TRACK-075); every row carries its company name, its usual driver (TRACK-034 D1), whether an expired blocking document stands against it (TRACK-016 D2), today''s route, its last fix, its soonest document due, the view counts over the other filters and the total_count the filters match; an empty page is one row with id NULL that still carries the counts';
