-- TRACK-043 rollback: no legs, no stored address, no shared stop.

DROP FUNCTION tracking.sp_create_rider(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, DECIMAL, DECIMAL, TEXT, VARCHAR, VARCHAR, UUID);
DROP FUNCTION tracking.sp_update_rider(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, UUID, DECIMAL, DECIMAL, TEXT, VARCHAR, VARCHAR, UUID);
DROP FUNCTION tracking.sp_get_rider(UUID, UUID, UUID, UUID);
DROP FUNCTION tracking.sp_list_riders(UUID, UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT, UUID, UUID, UUID, BOOLEAN, VARCHAR, BOOLEAN);

CREATE FUNCTION tracking.sp_create_rider(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_rider_type VARCHAR(50),
    p_first_name VARCHAR(100),
    p_last_name VARCHAR(100),
    p_identification_number VARCHAR(50),
    p_phone VARCHAR(20),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(20),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(20),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500),
    p_scope_user_id UUID DEFAULT NULL,
    p_home_latitude DECIMAL DEFAULT NULL,
    p_home_longitude DECIMAL DEFAULT NULL,
    p_notes TEXT DEFAULT NULL,
    p_group_label VARCHAR(100) DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    home_latitude DECIMAL,
    home_longitude DECIMAL,
    notes TEXT,
    group_label VARCHAR
) AS $$
DECLARE
    v_rider_id UUID;
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR o.id = ANY(v_scope))
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.riders (
        organization_id, rider_type, first_name, last_name,
        identification_number, phone, email, status, home_location, notes, group_label
    )
    VALUES (
        p_organization_id,
        COALESCE(NULLIF(TRIM(p_rider_type), ''), 'employee'),
        TRIM(p_first_name),
        TRIM(p_last_name),
        NULLIF(TRIM(p_identification_number), ''),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_email), ''),
        'active',
        CASE WHEN p_home_latitude IS NOT NULL AND p_home_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_home_longitude, p_home_latitude), 4326)::geography END,
        NULLIF(TRIM(p_notes), ''),
        NULLIF(TRIM(p_group_label), '')
    )
    RETURNING tracking.riders.id INTO v_rider_id;

    PERFORM tracking.fn_upsert_primary_rider_contact(
        v_rider_id, 'guardian', p_guardian_name, p_guardian_phone, p_guardian_email, p_guardian_user_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        v_rider_id, 'emergency', p_emergency_contact_name, p_emergency_contact_phone, NULL, NULL);

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
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
        CAST(p_address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(r.group_label AS VARCHAR)
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = v_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_update_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_phone VARCHAR(50),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(50),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(50),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500),
    p_is_active BOOLEAN,
    p_scope_user_id UUID DEFAULT NULL,
    p_home_latitude DECIMAL DEFAULT NULL,
    p_home_longitude DECIMAL DEFAULT NULL,
    p_notes TEXT DEFAULT NULL,
    p_group_label VARCHAR(100) DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    home_latitude DECIMAL,
    home_longitude DECIMAL,
    notes TEXT,
    group_label VARCHAR
) AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.riders r SET
        phone = NULLIF(TRIM(p_phone), ''),
        email = NULLIF(TRIM(p_email), ''),
        status = CASE
            WHEN p_is_active IS NOT NULL THEN CASE WHEN p_is_active THEN 'active' ELSE 'inactive' END
            ELSE r.status
        END,
        home_location = CASE WHEN p_home_latitude IS NOT NULL AND p_home_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_home_longitude, p_home_latitude), 4326)::geography END,
        notes = NULLIF(TRIM(p_notes), ''),
        -- NULL keeps the group, '' clears it (TRACK-039 D2).
        group_label = CASE WHEN p_group_label IS NULL THEN r.group_label ELSE NULLIF(TRIM(p_group_label), '') END,
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_rider_id;

    PERFORM tracking.fn_upsert_primary_rider_contact(
        p_rider_id, 'guardian', p_guardian_name, p_guardian_phone, p_guardian_email, p_guardian_user_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        p_rider_id, 'emergency', p_emergency_contact_name, p_emergency_contact_phone, NULL, NULL);

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
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
        CAST(p_address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(r.group_label AS VARCHAR)
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_list_riders(
    p_tenant_id UUID,
    p_organization_id UUID DEFAULT NULL,
    p_search VARCHAR DEFAULT NULL,
    p_rider_type VARCHAR DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20,
    p_scope_user_id UUID DEFAULT NULL,
    p_guardian_user_id UUID DEFAULT NULL,
    p_route_id UUID DEFAULT NULL,
    p_unassigned BOOLEAN DEFAULT NULL,
    p_group VARCHAR DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    organization_name VARCHAR,
    organization_kind VARCHAR,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    home_latitude DECIMAL,
    home_longitude DECIMAL,
    notes TEXT,
    group_label VARCHAR,
    assigned_route_name VARCHAR,
    total_count BIGINT
) AS $$
DECLARE
    v_scope UUID[];
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
        NULL::VARCHAR,
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(r.group_label AS VARCHAR),
        asg.route_name,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    -- The route the rider already rides in that direction on one of those days, this one first.
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
      AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope))
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
      AND (p_rider_type IS NULL OR p_rider_type = '' OR r.rider_type = p_rider_type)
      AND (p_is_active IS NULL OR (r.status = 'active') = p_is_active)
      AND (p_group IS NULL OR p_group = '' OR r.group_label = p_group)
      AND (NOT COALESCE(p_unassigned, false) OR asg.route_name IS NULL)
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
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_riders(UUID, UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT, UUID, UUID, UUID, BOOLEAN, VARCHAR) IS
'One page of a tenant''s riders by name with their organization''s name and kind, narrowed to p_scope_user_id''s organizations and to p_guardian_user_id''s own riders when either is set, and optionally by organization, type, status and a name/identification search; every row carries its group, the route it already rides in p_route_id''s direction (TRACK-039: p_unassigned keeps the riders with none, p_group one group) and the total_count the filters match';

CREATE FUNCTION tracking.sp_get_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_guardian_user_id UUID,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    home_latitude DECIMAL,
    home_longitude DECIMAL,
    notes TEXT,
    group_label VARCHAR
) AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
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
        NULL::VARCHAR,
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(r.group_label AS VARCHAR)
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_bulk_assign_riders(
    p_tenant_id  UUID,
    p_route_id   UUID,
    p_rider_ids  UUID[],
    p_days       SMALLINT DEFAULT NULL,
    p_valid_from DATE DEFAULT NULL
)
RETURNS TABLE(rider_id UUID, status VARCHAR, reason VARCHAR, warnings JSONB) AS $$
DECLARE
    v_days   SMALLINT;
    v_from   DATE;
    v_pair   UUID;
    v_rider  UUID;
    v_home   BOOLEAN;
    v_ids    UUID[] := '{}';
    v_status VARCHAR[] := '{}';
    v_reason VARCHAR[] := '{}';
BEGIN
    IF cardinality(COALESCE(p_rider_ids, '{}')) > 100 THEN
        RAISE EXCEPTION 'assignment.bulk-too-many' USING ERRCODE = 'P0001';
    END IF;
    SELECT r.paired_route_id, COALESCE(p_days, (
        SELECT CAST(bit_or(s.days_of_week) AS SMALLINT) FROM tracking.route_schedules s
        WHERE s.route_id = r.id AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
    ), 31)
      INTO v_pair, v_days
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id;
    IF v_days IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;
    v_from := COALESCE(p_valid_from, tracking.fn_route_today(p_route_id));

    FOREACH v_rider IN ARRAY COALESCE(p_rider_ids, '{}') LOOP
        SELECT rd.home_location IS NOT NULL INTO v_home
        FROM tracking.riders rd
        INNER JOIN tracking.organizations o ON o.id = rd.organization_id
        WHERE rd.id = v_rider AND o.tenant_id = p_tenant_id;

        v_ids := v_ids || v_rider;
        IF v_home IS NULL THEN
            v_status := v_status || CAST('skipped' AS VARCHAR);
            v_reason := v_reason || CAST('not-found' AS VARCHAR);
        ELSIF NOT v_home THEN
            v_status := v_status || CAST('skipped' AS VARCHAR);
            v_reason := v_reason || CAST('no-home' AS VARCHAR);
        ELSE
            BEGIN
                PERFORM 1 FROM tracking.sp_create_rider_route_assignments(p_tenant_id, jsonb_build_array(jsonb_build_object(
                    'rider_id', v_rider, 'route_id', p_route_id, 'days_of_week', v_days, 'valid_from', v_from)));
                v_status := v_status || CAST('assigned' AS VARCHAR);
                v_reason := v_reason || CAST(NULL AS VARCHAR);
            EXCEPTION WHEN raise_exception THEN
                v_status := v_status || CAST('skipped' AS VARCHAR);
                v_reason := v_reason || CAST(regexp_replace(SQLERRM, '^assignment\.', '') AS VARCHAR);
            END;
        END IF;
    END LOOP;

    RETURN QUERY
    SELECT u.rider_id, u.status, u.reason,
           tracking.fn_assignment_warnings(p_tenant_id, array_remove(ARRAY[p_route_id, v_pair], NULL))
    FROM unnest(v_ids, v_status, v_reason) WITH ORDINALITY AS u(rider_id, status, reason, ord)
    ORDER BY u.ord;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_bulk_assign_riders(UUID, UUID, UUID[], SMALLINT, DATE) IS
'Assigns up to 100 riders to a route (and its return) in one call, each in its own subtransaction: answers per rider assigned or skipped with its reason (no-home, overlap, not-found…) and the routes'' capacity warnings (TRACK-039 D1)';

CREATE OR REPLACE FUNCTION tracking.fn_route_home_stop(
    p_tenant_id UUID,
    p_route_id UUID,
    p_rider_id UUID,
    p_on DATE
)
RETURNS UUID AS $$
DECLARE
    v_home    GEOGRAPHY;
    v_name    VARCHAR;
    v_place   UUID;
    v_version RECORD;
    v_target  UUID;
    v_origin  GEOGRAPHY;
    v_dest    GEOGRAPHY;
    v_at      INT;
    v_added   BOOLEAN := false;
BEGIN
    SELECT rd.home_location, CAST(rd.first_name || ' ' || rd.last_name AS VARCHAR)
      INTO v_home, v_name
    FROM tracking.riders rd
    WHERE rd.id = p_rider_id;
    IF v_home IS NULL THEN
        RETURN NULL;
    END IF;

    SELECT sp.id INTO v_place
    FROM tracking.stop_places sp
    WHERE sp.tenant_id = p_tenant_id
      AND sp.location IS NOT NULL
      AND ST_DWithin(sp.location, v_home, 30)
    ORDER BY ST_Distance(sp.location, v_home)
    LIMIT 1;
    IF v_place IS NULL THEN
        INSERT INTO tracking.stop_places (tenant_id, name, location)
        VALUES (p_tenant_id, v_name, v_home)
        RETURNING tracking.stop_places.id INTO v_place;
    END IF;

    SELECT
        CASE WHEN r.origin_lat IS NOT NULL AND r.origin_lng IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(r.origin_lng, r.origin_lat), 4326)::geography END,
        CASE WHEN r.destination_lat IS NOT NULL AND r.destination_lng IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(r.destination_lng, r.destination_lat), 4326)::geography END
      INTO v_origin, v_dest
    FROM tracking.routes r
    WHERE r.id = p_route_id;

    FOR v_version IN
        SELECT rv.* FROM tracking.route_versions rv
        WHERE rv.route_id = p_route_id
          AND (rv.effective_to IS NULL OR rv.effective_to >= p_on)
        ORDER BY rv.effective_from
    LOOP
        IF EXISTS (
            SELECT 1 FROM tracking.route_version_stops vs
            WHERE vs.version_id = v_version.id AND vs.stop_place_id = v_place
        ) THEN
            CONTINUE;
        END IF;

        v_target := v_version.id;
        IF v_version.effective_from < p_on THEN
            UPDATE tracking.route_versions rv
               SET effective_to = p_on - 1, updated_at = CURRENT_TIMESTAMP
             WHERE rv.id = v_version.id;
            INSERT INTO tracking.route_versions (route_id, effective_from, effective_to, created_by)
            VALUES (p_route_id, p_on, v_version.effective_to, v_version.created_by)
            RETURNING tracking.route_versions.id INTO v_target;
            INSERT INTO tracking.route_version_stops (version_id, stop_place_id, sequence, planned_offset_min, dwell_sec)
            SELECT v_target, vs.stop_place_id, vs.sequence, vs.planned_offset_min, vs.dwell_sec
            FROM tracking.route_version_stops vs
            WHERE vs.version_id = v_version.id;
        END IF;

        -- The cheapest gap: each candidate sequence k sits between stop k-1 (the route's origin before
        -- the first) and stop k (its destination after the last); a missing end costs nothing.
        WITH s AS (
            SELECT vs.sequence, sp.location
            FROM tracking.route_version_stops vs
            INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id
            WHERE vs.version_id = v_target
        ),
        n AS (SELECT COALESCE(MAX(s.sequence), 0) AS last FROM s),
        gaps AS (
            SELECT k,
                   COALESCE(prev.location, CASE WHEN k = 1 THEN v_origin END) AS a,
                   COALESCE(nxt.location, CASE WHEN k = n.last + 1 THEN v_dest END) AS b
            FROM n
            CROSS JOIN generate_series(1, n.last + 1) k
            LEFT JOIN s prev ON prev.sequence = k - 1
            LEFT JOIN s nxt ON nxt.sequence = k
        )
        SELECT g.k INTO v_at
        FROM gaps g
        ORDER BY
            COALESCE(ST_Distance(g.a, v_home), 0) + COALESCE(ST_Distance(v_home, g.b), 0)
            - COALESCE(ST_Distance(g.a, g.b), 0),
            g.k DESC
        LIMIT 1;

        -- Two passes keep UNIQUE (version_id, sequence) and sequence >= 1 true after each statement.
        UPDATE tracking.route_version_stops vs SET sequence = vs.sequence + 100000
        WHERE vs.version_id = v_target AND vs.sequence >= v_at;
        UPDATE tracking.route_version_stops vs SET sequence = vs.sequence - 99999, updated_at = CURRENT_TIMESTAMP
        WHERE vs.version_id = v_target AND vs.sequence > 100000;
        INSERT INTO tracking.route_version_stops (version_id, stop_place_id, sequence)
        VALUES (v_target, v_place, v_at);
        v_added := true;
    END LOOP;

    -- Every trip from p_on on calls at one more stop, and the line is re-traced.
    IF v_added THEN
        PERFORM tracking.fn_route_changed(p_route_id, p_on, NULL);
    END IF;
    RETURN v_place;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_route_home_stop(UUID, UUID, UUID, DATE) IS
'Puts the rider''s home (a place within 30 m, or a new one) on every version of the route in force from p_on, at the cheapest straight-line position, splitting a version that started earlier; NULL when the rider has no home pin (TRACK-037 D1)';

DROP FUNCTION tracking.fn_rider_stop_place(UUID, UUID);
DROP INDEX tracking.idx_riders_missing_location;
ALTER TABLE tracking.riders DROP CONSTRAINT chk_rider_service_legs, DROP COLUMN stop_place_id, DROP COLUMN service_legs, DROP COLUMN address;
