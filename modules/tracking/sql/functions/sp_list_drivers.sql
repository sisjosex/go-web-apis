CREATE OR REPLACE FUNCTION tracking.sp_list_drivers(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_company_id uuid DEFAULT NULL::uuid, p_status character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, company_id uuid, company_name character varying, user_id uuid, user_email character varying, first_name character varying, last_name character varying, phone character varying, license_number character varying, license_class character varying, license_expires_on date, status character varying, service_blocked boolean, created_at timestamp without time zone, updated_at timestamp without time zone, today_route_name character varying, app_status character varying, summary jsonb, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    -- TRACK-074: the stat cards read every driver the filters keep, so the three flags are one
    -- indexed probe each over all of them; the route name and the rest only for the page.
    WITH f AS MATERIALIZED (
        SELECT
            d.id,
            d.last_name,
            d.first_name,
            CAST(tc.name AS VARCHAR) AS company_name,
            -- D1: an account that synced an op or started a trip in the last 7 days.
            CASE
                WHEN d.user_id IS NULL THEN 'none'
                WHEN EXISTS (
                    SELECT 1 FROM tracking.driver_ops op
                    WHERE op.tenant_id = p_tenant_id AND op.driver_id = d.id
                      AND op.received_at >= now() - INTERVAL '7 days'
                ) OR EXISTS (
                    SELECT 1 FROM tracking.trips t
                    WHERE t.tenant_id = p_tenant_id AND t.service_date >= CURRENT_DATE - 8
                      AND t.driver_id = d.id AND t.started_at >= now() - INTERVAL '7 days'
                ) THEN 'active'
                ELSE 'invited'
            END AS app_status,
            (d.license_expires_on IS NOT NULL AND d.license_expires_on <= CURRENT_DATE + 30)
            OR EXISTS (
                SELECT 1 FROM tracking.compliance_documents doc
                INNER JOIN tracking.document_types dt ON dt.id = doc.document_type_id
                WHERE doc.tenant_id = p_tenant_id AND doc.subject_type = 'driver' AND doc.subject_id = d.id
                  AND dt.is_active AND doc.expires_on IS NOT NULL
                  AND doc.expires_on <= CURRENT_DATE + dt.warn_days_before
            ) AS docs_due,
            NOT EXISTS (
                SELECT 1 FROM tracking.trips t
                WHERE t.tenant_id = p_tenant_id AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
                  AND t.driver_id = d.id
                  AND (t.status = 'in_progress'
                       OR (t.status = 'planned' AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)))
            ) AS without_route_today
        FROM tracking.drivers d
        INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
        WHERE d.tenant_id = p_tenant_id
          -- TRACK-077: one driver's row, so its page reads the same figures as the list.
          AND (p_id IS NULL OR d.id = p_id)
          AND (p_company_id IS NULL OR d.company_id = p_company_id)
          AND (p_status IS NULL OR p_status = '' OR d.status = p_status)
          AND (p_search IS NULL OR p_search = ''
               OR d.first_name ILIKE '%' || p_search || '%'
               OR d.last_name ILIKE '%' || p_search || '%'
               OR d.license_number ILIKE '%' || p_search || '%')
    ),
    s AS (
        SELECT jsonb_build_object(
            'total',               COUNT(*),
            'app_active',          COUNT(*) FILTER (WHERE f.app_status = 'active'),
            'docs_due',            COUNT(*) FILTER (WHERE f.docs_due),
            'without_route_today', COUNT(*) FILTER (WHERE f.without_route_today)
        ) AS summary
        FROM f
    ),
    pg AS (
        SELECT f.id, f.company_name, f.app_status, CAST(COUNT(*) OVER () AS BIGINT) AS total_count
        FROM f
        ORDER BY f.last_name ASC, f.first_name ASC, f.id ASC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        d.id,
        d.tenant_id,
        d.company_id,
        pg.company_name,
        d.user_id,
        CAST(u.email AS VARCHAR),
        CAST(d.first_name AS VARCHAR),
        CAST(d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(d.license_number AS VARCHAR),
        CAST(d.license_class AS VARCHAR),
        d.license_expires_on,
        CAST(d.status AS VARCHAR),
        CASE WHEN d.id IS NOT NULL THEN tracking.fn_service_blocked(d.tenant_id, 'driver', d.id) END,
        d.created_at,
        d.updated_at,
        CAST(tr.route_name AS VARCHAR),
        CAST(pg.app_status AS VARCHAR),
        s.summary,
        COALESCE(pg.total_count, 0)
    -- An empty page still answers one row, id NULL, so the stat cards keep their figures.
    FROM s
    LEFT JOIN pg ON true
    LEFT JOIN tracking.drivers d ON d.id = pg.id
    LEFT JOIN auth.users u ON u.id = d.user_id AND u.deleted_at IS NULL
    -- The trip in progress, else the next one planned on its own local today.
    LEFT JOIN LATERAL (
        SELECT r.route_name FROM tracking.trips t
        INNER JOIN tracking.routes r ON r.id = t.route_id
        WHERE t.tenant_id = p_tenant_id AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
          AND t.driver_id = d.id
          AND (t.status = 'in_progress'
               OR (t.status = 'planned' AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)))
        ORDER BY (t.status = 'in_progress') DESC, t.planned_start ASC
        LIMIT 1
    ) tr ON true
    ORDER BY d.last_name ASC, d.first_name ASC, d.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_drivers(uuid,character varying,uuid,character varying,integer,integer,uuid) IS 'One page of a tenant''s drivers by surname, optionally narrowed by a name or licence search, carrier and status, or to one driver (p_id, TRACK-077); every row carries its carrier, its linked account''s email, whether an expired blocking document stands against it (TRACK-016 D2), today''s route and whether it uses the app (TRACK-074 D1), the summary over every driver the filters match and the total_count; an empty page is one row with id NULL that still carries the summary';
