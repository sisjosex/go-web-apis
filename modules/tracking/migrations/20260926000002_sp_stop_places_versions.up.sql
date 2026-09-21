-- TRACK-007 steps 2–4: the stop-places slice, the route-version slice, and the `route.changed`
-- outbox row every one of their writes leaves behind.
--
-- Stop places are read two ways: by name, and by "what is near this point". The second is what the
-- map editor asks on every pan, so it goes through the GIST index — ST_DWithin on geography is
-- metres and index-backed, where a hand-rolled haversine would be a scan of the whole table.
--
-- A version is published, never edited in place once it is in force: `sp_create_route_version`
-- closes the previous one the day before the new one starts, so the two together always cover the
-- calendar without a gap and never overlap (the table's exclusion constraint enforces the second
-- half of that, so a concurrent publish cannot slip through between the check and the insert).

-- ===========================================================================
-- Stop places
-- ===========================================================================
CREATE FUNCTION tracking.sp_create_stop_place(
    p_tenant_id UUID,
    p_name VARCHAR(255),
    p_address VARCHAR(500),
    p_latitude DOUBLE PRECISION,
    p_longitude DOUBLE PRECISION,
    p_organization_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    organization_id UUID,
    name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_id UUID;
BEGIN
    IF p_organization_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.stop_places (tenant_id, organization_id, name, address, location)
    VALUES (
        p_tenant_id,
        p_organization_id,
        TRIM(p_name),
        NULLIF(TRIM(p_address), ''),
        ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography
    )
    RETURNING tracking.stop_places.id INTO v_id;

    RETURN QUERY
    SELECT
        sp.id,
        sp.tenant_id,
        sp.organization_id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        sp.created_at,
        sp.updated_at
    FROM tracking.stop_places sp
    WHERE sp.id = v_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_stop_place(UUID, VARCHAR, VARCHAR, DOUBLE PRECISION, DOUBLE PRECISION, UUID) IS
'Creates a stop place in a tenant; raises tracking.organization.not-found when the organization it is pinned to is not the tenant''s';

-- A NULL argument keeps the stored value, so a PATCH sending one field changes one field. Latitude
-- and longitude move together or not at all: half a coordinate is not a place.
CREATE FUNCTION tracking.sp_update_stop_place(
    p_tenant_id UUID,
    p_stop_place_id UUID,
    p_name VARCHAR(255),
    p_address VARCHAR(500),
    p_latitude DOUBLE PRECISION,
    p_longitude DOUBLE PRECISION,
    p_organization_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    organization_id UUID,
    name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.stop_places sp
        WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_organization_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.stop_places sp SET
        name            = COALESCE(NULLIF(TRIM(p_name), ''), sp.name),
        address         = COALESCE(NULLIF(TRIM(p_address), ''), sp.address),
        organization_id = COALESCE(p_organization_id, sp.organization_id),
        location        = CASE
            WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography
            ELSE sp.location
        END,
        updated_at = CURRENT_TIMESTAMP
    WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id;

    -- Moving or renaming a place changes every route that calls there, from today on: one row per
    -- affected route, in this transaction (INFRA-001 D3). A place no version names changes nothing,
    -- so it writes nothing.
    INSERT INTO tracking.outbox (topic, payload)
    SELECT DISTINCT
        'route.changed',
        jsonb_build_object(
            'route_id',  rv.route_id,
            'date_from', CURRENT_DATE,
            'date_to',   CURRENT_DATE
        )
    FROM tracking.route_version_stops vs
    INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
    WHERE vs.stop_place_id = p_stop_place_id;

    RETURN QUERY
    SELECT
        sp.id,
        sp.tenant_id,
        sp.organization_id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        sp.created_at,
        sp.updated_at
    FROM tracking.stop_places sp
    WHERE sp.id = p_stop_place_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_stop_place(UUID, UUID, VARCHAR, VARCHAR, DOUBLE PRECISION, DOUBLE PRECISION, UUID) IS
'Edits a stop place; a NULL argument keeps the stored value and the coordinate moves only when both halves are sent';

-- p_latitude/p_longitude set turn the list into "what is near this point": ST_DWithin narrows
-- through the GIST index, distance_m rides on every row and the order is nearest first. Without
-- them the answer is the tenant's stop places by name, and distance_m is NULL.
CREATE FUNCTION tracking.sp_list_stop_places(
    p_tenant_id UUID,
    p_search    VARCHAR DEFAULT NULL,
    p_latitude  DOUBLE PRECISION DEFAULT NULL,
    p_longitude DOUBLE PRECISION DEFAULT NULL,
    p_radius_m  INT DEFAULT NULL,
    p_page      INT DEFAULT 1,
    p_page_size INT DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    organization_id UUID,
    name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    distance_m DECIMAL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) AS $$
DECLARE
    v_point  GEOGRAPHY;
    v_radius INT := COALESCE(p_radius_m, 1000);
BEGIN
    IF p_latitude IS NOT NULL AND p_longitude IS NOT NULL THEN
        v_point := ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography;
    END IF;

    RETURN QUERY
    SELECT
        sp.id,
        sp.tenant_id,
        sp.organization_id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        CASE WHEN v_point IS NULL THEN NULL ELSE CAST(ST_Distance(sp.location, v_point) AS DECIMAL) END,
        sp.created_at,
        sp.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.stop_places sp
    WHERE sp.tenant_id = p_tenant_id
      AND (p_search IS NULL OR p_search = '' OR sp.name ILIKE '%' || p_search || '%')
      AND (v_point IS NULL OR (sp.location IS NOT NULL AND ST_DWithin(sp.location, v_point, v_radius)))
    -- Nearest first when a point was given; alphabetical otherwise, with sp.id breaking ties so the
    -- page boundary follows the order the reader sees.
    ORDER BY
        CASE WHEN v_point IS NULL THEN NULL ELSE ST_Distance(sp.location, v_point) END ASC NULLS LAST,
        sp.name ASC,
        sp.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_stop_places(UUID, VARCHAR, DOUBLE PRECISION, DOUBLE PRECISION, INT, INT, INT) IS
'One page of a tenant''s stop places, by name or — when a point is given — within p_radius_m of it, nearest first and carrying distance_m; every row carries the total_count the filters match';

CREATE FUNCTION tracking.sp_get_stop_place(
    p_tenant_id UUID,
    p_stop_place_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    organization_id UUID,
    name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.stop_places sp
        WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        sp.id,
        sp.tenant_id,
        sp.organization_id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        sp.created_at,
        sp.updated_at
    FROM tracking.stop_places sp
    WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

-- A stop place a version still names is never deleted: the version is the record of what a route
-- ran, and a stop that vanishes from it rewrites history. The operator takes it off the list first.
CREATE FUNCTION tracking.sp_delete_stop_place(
    p_tenant_id UUID,
    p_stop_place_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_deleted INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.stop_places sp
        WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.route_version_stops vs WHERE vs.stop_place_id = p_stop_place_id
    ) THEN
        RAISE EXCEPTION 'stop-place.in-use' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.stop_places sp
    WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted > 0;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_stop_place(UUID, UUID) IS
'Deletes a stop place; raises tracking.stop-place.in-use while any route version still names it';

-- ===========================================================================
-- Route versions
-- ===========================================================================

-- fn_route_version_stops_json is the one place that reads the editor's list. The client's `sequence`
-- is a sort key, not the stored number: the rows are renumbered 1..n on the way in, so a whole-list
-- reorder cannot collide with uk_route_version_stop_sequence however the caller numbered it.
CREATE FUNCTION tracking.fn_route_version_stops_json(
    p_tenant_id UUID,
    p_version_id UUID,
    p_stops JSONB
)
RETURNS INT AS $$
DECLARE
    v_inserted INT;
BEGIN
    IF EXISTS (
        SELECT 1
        FROM jsonb_array_elements(COALESCE(p_stops, '[]'::jsonb)) e
        WHERE NOT EXISTS (
            SELECT 1 FROM tracking.stop_places sp
            WHERE sp.id = (e->>'stop_place_id')::UUID
              AND sp.tenant_id = p_tenant_id
        )
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_version_stops (version_id, stop_place_id, sequence, planned_offset_min, dwell_sec)
    SELECT
        p_version_id,
        (e.value->>'stop_place_id')::UUID,
        CAST(ROW_NUMBER() OVER (ORDER BY COALESCE((e.value->>'sequence')::INT, CAST(e.ordinality AS INT)), e.ordinality) AS INT),
        (e.value->>'planned_offset_min')::INT,
        (e.value->>'dwell_sec')::INT
    FROM jsonb_array_elements(COALESCE(p_stops, '[]'::jsonb)) WITH ORDINALITY AS e(value, ordinality);
    GET DIAGNOSTICS v_inserted = ROW_COUNT;
    RETURN v_inserted;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_route_version_stops_json(UUID, UUID, JSONB) IS
'Writes an editor''s stop list onto a version, renumbering it 1..n in the order the caller asked for; raises tracking.stop-place.not-found for a place outside the tenant';

CREATE FUNCTION tracking.sp_list_route_versions(
    p_tenant_id UUID,
    p_route_id UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    effective_from DATE,
    effective_to DATE,
    stops_count INT,
    created_by UUID,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rv.id,
        rv.route_id,
        rv.effective_from,
        rv.effective_to,
        CAST(COUNT(vs.id) AS INT),
        rv.created_by,
        rv.created_at,
        rv.updated_at
    FROM tracking.route_versions rv
    LEFT JOIN tracking.route_version_stops vs ON vs.version_id = rv.id
    WHERE rv.route_id = p_route_id
    GROUP BY rv.id, rv.route_id, rv.effective_from, rv.effective_to, rv.created_by, rv.created_at, rv.updated_at
    -- Newest first: the version an operator opens the screen to change is the one in force.
    ORDER BY rv.effective_from DESC, rv.id DESC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_route_versions(UUID, UUID) IS
'A route''s stop lists over time, newest first, each with how many stops it holds; raises tracking.route.not-found outside the tenant';

-- Publishing a version closes the one in force the day before the new one starts, so the two cover
-- the calendar without a gap. A version that does not start after the latest is refused: history is
-- appended to, never spliced, and the exclusion constraint on the table is the backstop for two
-- publishes racing each other.
CREATE FUNCTION tracking.sp_create_route_version(
    p_tenant_id UUID,
    p_route_id UUID,
    p_effective_from DATE,
    p_stops JSONB,
    p_created_by UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    effective_from DATE,
    effective_to DATE,
    stops_count INT,
    created_by UUID,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_latest     DATE;
    v_version_id UUID;
    v_stops      INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT MAX(rv.effective_from) INTO v_latest
    FROM tracking.route_versions rv
    WHERE rv.route_id = p_route_id;

    IF v_latest IS NOT NULL AND p_effective_from <= v_latest THEN
        RAISE EXCEPTION 'route.version-overlap' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.route_versions rv
       SET effective_to = p_effective_from - 1,
           updated_at   = CURRENT_TIMESTAMP
     WHERE rv.route_id = p_route_id
       AND rv.effective_to IS NULL;

    INSERT INTO tracking.route_versions (route_id, effective_from, effective_to, created_by)
    VALUES (p_route_id, p_effective_from, NULL, p_created_by)
    RETURNING tracking.route_versions.id INTO v_version_id;

    v_stops := tracking.fn_route_version_stops_json(p_tenant_id, v_version_id, p_stops);

    -- The route runs a different list from p_effective_from onwards, with no end: date_to NULL is
    -- "and every day after". Written here so the row exists if and only if the publish committed.
    INSERT INTO tracking.outbox (topic, payload)
    VALUES ('route.changed', jsonb_build_object(
        'route_id',  p_route_id,
        'date_from', p_effective_from,
        'date_to',   NULL
    ));

    RETURN QUERY
    SELECT
        rv.id,
        rv.route_id,
        rv.effective_from,
        rv.effective_to,
        v_stops,
        rv.created_by,
        rv.created_at,
        rv.updated_at
    FROM tracking.route_versions rv
    WHERE rv.id = v_version_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_route_version(UUID, UUID, DATE, JSONB, UUID) IS
'Publishes a route''s next stop list from p_effective_from, closing the one in force the day before; raises tracking.route.version-overlap when it would not start after the latest';

-- The whole-list reorder: one request replaces the list, so the editor never renumbers rows one at
-- a time and never leaves a half-applied order behind. A version whose first day has passed is the
-- record of what the route ran and is refused — changing it from here on is a new version.
CREATE FUNCTION tracking.sp_replace_route_version_stops(
    p_tenant_id UUID,
    p_version_id UUID,
    p_stops JSONB
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    version_id UUID,
    stop_place_id UUID,
    stop_name VARCHAR,
    address VARCHAR,
    latitude DECIMAL,
    longitude DECIMAL,
    sequence INT,
    planned_offset_min INT,
    dwell_sec INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_effective_from DATE;
BEGIN
    SELECT rv.effective_from INTO v_effective_from
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.id = p_version_id AND tc.tenant_id = p_tenant_id;

    IF v_effective_from IS NULL THEN
        RAISE EXCEPTION 'route.version-not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_effective_from < CURRENT_DATE THEN
        RAISE EXCEPTION 'route.version-closed' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.route_version_stops vs WHERE vs.version_id = p_version_id;
    PERFORM tracking.fn_route_version_stops_json(p_tenant_id, p_version_id, p_stops);

    UPDATE tracking.route_versions rv
       SET updated_at = CURRENT_TIMESTAMP
     WHERE rv.id = p_version_id;

    INSERT INTO tracking.outbox (topic, payload)
    SELECT 'route.changed', jsonb_build_object(
        'route_id',  rv.route_id,
        'date_from', rv.effective_from,
        'date_to',   NULL
    )
    FROM tracking.route_versions rv
    WHERE rv.id = p_version_id;

    RETURN QUERY
    SELECT
        vs.id,
        rv.route_id,
        rv.id,
        sp.id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        vs.sequence,
        vs.planned_offset_min,
        vs.dwell_sec,
        vs.created_at,
        vs.updated_at
    FROM tracking.route_version_stops vs
    INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
    INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id
    WHERE vs.version_id = p_version_id
    ORDER BY vs.sequence ASC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_replace_route_version_stops(UUID, UUID, JSONB) IS
'Replaces a route version''s whole stop list in one statement and returns it; raises tracking.route.version-closed once the version''s first day has passed';
