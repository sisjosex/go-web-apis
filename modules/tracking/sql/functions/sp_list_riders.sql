CREATE OR REPLACE FUNCTION tracking.sp_list_riders(p_company_id uuid, p_guardian_user_id uuid, p_rider_type character varying, p_is_active boolean)
 RETURNS TABLE(id uuid, company_id uuid, rider_type character varying, first_name character varying, last_name character varying, identification_number character varying, phone character varying, email character varying, emergency_contact_name character varying, emergency_contact_phone character varying, guardian_user_id uuid, guardian_name character varying, guardian_phone character varying, guardian_email character varying, address character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        tracking.riders.id,
        tracking.riders.company_id,
        NULL::VARCHAR(50),
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        (tracking.riders.emergency_contact->>'name')::VARCHAR(255),
        (tracking.riders.emergency_contact->>'phone')::VARCHAR(50),
        NULL::UUID,
        NULL::VARCHAR(255),
        NULL::VARCHAR(50),
        NULL::VARCHAR(255),
        NULL::VARCHAR(500),
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at
    FROM tracking.riders
    WHERE (p_company_id IS NULL OR tracking.riders.company_id = p_company_id)
      AND (p_is_active IS NULL OR (tracking.riders.status = 'active') = p_is_active)
    ORDER BY tracking.riders.created_at DESC;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_list_riders(p_tenant_id uuid, p_organization_id uuid DEFAULT NULL::uuid, p_search character varying DEFAULT NULL::character varying, p_rider_type character varying DEFAULT NULL::character varying, p_is_active boolean DEFAULT NULL::boolean, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid, p_route_id uuid DEFAULT NULL::uuid, p_unassigned boolean DEFAULT NULL::boolean, p_group character varying DEFAULT NULL::character varying, p_missing_location boolean DEFAULT NULL::boolean, p_geocoded boolean DEFAULT NULL::boolean)
 RETURNS TABLE(id uuid, organization_id uuid, organization_name character varying, organization_kind character varying, rider_type character varying, first_name character varying, last_name character varying, identification_number character varying, phone character varying, email character varying, emergency_contact_name character varying, emergency_contact_phone character varying, guardian_user_id uuid, guardian_name character varying, guardian_phone character varying, guardian_email character varying, address character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, home_latitude numeric, home_longitude numeric, notes text, group_label character varying, assigned_route_name character varying, service_legs character varying, stop_place_id uuid, stop_place_name character varying, home_geocoded boolean, total_count bigint)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_riders UUID[];
    v_guardian_scope UUID[];
    v_direction VARCHAR;
    v_days SMALLINT;
BEGIN
    -- The route the bulk add is for (TRACK-039 D1): its direction and the days it runs decide which
    -- assignment would collide.
    IF p_route_id IS NOT NULL THEN
        SELECT r.direction, COALESCE((
            SELECT CAST(bit_or(s.days_of_week) AS SMALLINT) FROM tracking.route_schedules s
            WHERE s.route_id = r.id AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
        ), 31)
          INTO v_direction, v_days
        FROM tracking.routes r WHERE r.id = p_route_id;
    END IF;

    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    END IF;

    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(o.name AS VARCHAR),
        CAST(o.kind AS VARCHAR),
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        CAST(r.address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(r.group_label AS VARCHAR),
        asg.route_name,
        CAST(r.service_legs AS VARCHAR),
        r.stop_place_id,
        CAST(sp.name AS VARCHAR),
        r.home_geocoded,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    -- The route the rider already rides in that direction on one of those days, this one first.
    LEFT JOIN tracking.stop_places sp ON sp.id = r.stop_place_id
    LEFT JOIN LATERAL (
        SELECT CAST(ar.route_name AS VARCHAR) AS route_name
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes ar ON ar.id = a.route_id
        WHERE v_direction IS NOT NULL
          AND a.rider_id = r.id
          AND ar.direction = v_direction
          AND COALESCE(a.valid_until, CURRENT_DATE) >= CURRENT_DATE
          AND (a.days_of_week & v_days) <> 0
        ORDER BY (a.route_id = p_route_id) DESC, a.valid_from
        LIMIT 1
    ) asg ON true
    WHERE o.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
      AND (v_riders IS NULL OR r.id = ANY(v_riders))
      AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope))
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
      AND (p_rider_type IS NULL OR p_rider_type = '' OR r.rider_type = p_rider_type)
      AND (p_is_active IS NULL OR (r.status = 'active') = p_is_active)
      AND (p_group IS NULL OR p_group = '' OR r.group_label = p_group)
      AND (NOT COALESCE(p_unassigned, false) OR asg.route_name IS NULL)
      AND (NOT COALESCE(p_missing_location, false) OR r.home_location IS NULL)
      AND (NOT COALESCE(p_geocoded, false) OR r.home_geocoded)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR r.first_name            ILIKE '%' || p_search || '%'
          OR r.last_name             ILIKE '%' || p_search || '%'
          OR r.identification_number ILIKE '%' || p_search || '%'
      )
    -- Read alphabetically, so the page is cut the same way the reader sees it; r.id breaks ties.
    ORDER BY r.last_name ASC, r.first_name ASC, r.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_riders(uuid,uuid,character varying,character varying,boolean,integer,integer,uuid,uuid,uuid,boolean,character varying,boolean,boolean) IS 'One page of a tenant''s riders by name with their organization''s name and kind, narrowed to p_scope_user_id''s organizations and to p_guardian_user_id''s own riders when either is set, and optionally by organization, type, status and a name/identification search; every row carries its group, the route it already rides in p_route_id''s direction (TRACK-039: p_unassigned keeps the riders with none, p_group one group, p_missing_location the riders without a pin — TRACK-043, p_geocoded those the import placed by address — TRACK-072) and the total_count the filters match';
