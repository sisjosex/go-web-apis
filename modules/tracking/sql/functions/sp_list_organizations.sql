CREATE OR REPLACE FUNCTION tracking.sp_list_organizations(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_kind character varying DEFAULT NULL::character varying, p_is_active boolean DEFAULT NULL::boolean, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_scope_user_id uuid DEFAULT NULL::uuid, p_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, kind character varying, name character varying, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, absence_cutoff_min integer, latitude numeric, longitude numeric, address character varying, stop_place_id uuid, empty_trip_cancel_min integer, delay_notice_min integer, my_role character varying, my_groups text[], routes_count bigint, riders_count bigint, absent_today bigint, shifts_count bigint, counts jsonb, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    -- TRACK-074: the chips count the kinds over every other filter; the card figures are read for the
    -- page's rows only.
    WITH f AS MATERIALIZED (
        SELECT o.id, o.name, o.kind, CAST(me.role AS VARCHAR) AS my_role, me.groups AS my_groups
        FROM tracking.organizations o
        -- TRACK-067 D1: an organization user lists only its own organizations, each with its role there;
        -- staff (p_scope_user_id NULL) join nothing and read every row with my_role NULL.
        LEFT JOIN tracking.organization_members me ON me.organization_id = o.id AND me.user_id = p_scope_user_id
        WHERE o.tenant_id = p_tenant_id
          -- TRACK-077: one organization's row, so its page reads the same figures as the list.
          AND (p_id IS NULL OR o.id = p_id)
          AND (p_scope_user_id IS NULL OR me.user_id IS NOT NULL)
          AND (p_search IS NULL OR p_search = '' OR o.name ILIKE '%' || p_search || '%')
          AND (p_is_active IS NULL OR o.is_active = p_is_active)
    ),
    c AS (
        SELECT jsonb_build_object(
            'all',     COUNT(*),
            'school',  COUNT(*) FILTER (WHERE f.kind = 'school'),
            'company', COUNT(*) FILTER (WHERE f.kind = 'company'),
            'other',   COUNT(*) FILTER (WHERE f.kind = 'other')
        ) AS counts
        FROM f
    ),
    pg AS (
        SELECT f.id, f.my_role, f.my_groups, CAST(COUNT(*) OVER () AS BIGINT) AS total_count
        FROM f
        WHERE p_kind IS NULL OR p_kind = '' OR f.kind = p_kind
        -- Alphabetical, so the page boundary follows the order the reader sees; f.id breaks ties.
        ORDER BY f.name ASC, f.id ASC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        fr.*,
        pg.my_role,
        pg.my_groups,
        (SELECT COUNT(*) FROM tracking.routes r WHERE r.organization_id = fr.id AND r.is_active),
        (SELECT COUNT(*) FROM tracking.riders x WHERE x.organization_id = fr.id AND x.status = 'active'),
        -- The riders an absence covers on the organization's own local today.
        (SELECT COUNT(*) FROM tracking.riders x
         WHERE x.organization_id = fr.id AND x.status = 'active'
           AND EXISTS (
               SELECT 1 FROM tracking.rider_absences ab
               WHERE ab.rider_id = x.id
                 AND CAST(now() AT TIME ZONE fr.timezone AS DATE) BETWEEN ab.date_from AND ab.date_to)),
        -- A shift is a departure time: the distinct start times of its routes' schedules in force.
        (SELECT COUNT(DISTINCT s.start_time) FROM tracking.route_schedules s
         INNER JOIN tracking.routes r ON r.id = s.route_id
         WHERE r.organization_id = fr.id AND r.is_active
           AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE),
        c.counts,
        COALESCE(pg.total_count, 0)
    -- An empty page still answers one row, id NULL, so the chips keep their counts.
    FROM c
    LEFT JOIN pg ON true
    LEFT JOIN LATERAL tracking.fn_organization_row(p_tenant_id, pg.id) fr ON true
    ORDER BY fr.name ASC, fr.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_organizations(uuid,character varying,character varying,boolean,integer,integer,uuid,uuid) IS 'One page of a tenant''s organizations by name, with their location (TRACK-038 D2) and stop (TRACK-045 D3), optionally narrowed by a name search, kind and status, or to one organization (p_id, TRACK-077); an organization user (p_scope_user_id) sees only its own, with my_role and my_groups (TRACK-067); every row carries its active routes and riders, today''s absent riders and its shifts (TRACK-074), the kind counts over the other filters and the total_count the filters match; an empty page is one row with id NULL that still carries the counts';
