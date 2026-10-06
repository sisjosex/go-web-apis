-- TRACK-045: a client's location is its stop.
--
-- D2  POST /tracking/organizations requires the pin (the DTO binds it); the SPs keep accepting NULL so
--     an old client without one still reads and edits.
-- D3  organizations.stop_place_id is the client's own stop: created (or one already pinned to the
--     client within 30 m reused) when the client is saved with a pin and p_create_stop, moved and
--     renamed with the client, and the destination every pair of the client calls at instead of a copy.

ALTER TABLE tracking.organizations
    ADD COLUMN stop_place_id UUID REFERENCES tracking.stop_places(id) ON DELETE SET NULL;

COMMENT ON COLUMN tracking.organizations.stop_place_id IS
'The client''s own stop at its pin: the destination of its route pairs, kept in sync with the pin (TRACK-045 D3)';

-- ===========================================================================
-- The client's stop, kept in sync
-- ===========================================================================

-- Links or follows the client's stop. With a stop: moves and renames it when the client's pin, name or
-- address changed (sp_update_stop_place writes route.changed for every route calling there). Without
-- one and p_create: reuses the nearest stop within 30 m already pinned to the client, otherwise
-- inserts one, and links it. A public stop is never claimed: it may be a rider's home, which must not
-- move with the client. A client without a pin is left alone.
CREATE FUNCTION tracking.fn_organization_stop_sync(p_tenant_id UUID, p_organization_id UUID, p_create BOOLEAN)
RETURNS UUID AS $$
DECLARE
    v_org   RECORD;
    v_stop  RECORD;
    v_place UUID;
BEGIN
    SELECT o.name, o.address, o.location, o.stop_place_id INTO v_org
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF v_org.location IS NULL THEN
        RETURN v_org.stop_place_id;
    END IF;

    IF v_org.stop_place_id IS NOT NULL THEN
        SELECT sp.name, sp.address, sp.location INTO v_stop
        FROM tracking.stop_places sp WHERE sp.id = v_org.stop_place_id;
        IF v_stop.location IS DISTINCT FROM v_org.location
            OR v_stop.name IS DISTINCT FROM v_org.name
            OR (v_org.address IS NOT NULL AND v_stop.address IS DISTINCT FROM v_org.address) THEN
            PERFORM tracking.sp_update_stop_place(p_tenant_id, v_org.stop_place_id, v_org.name, v_org.address,
                ST_Y(v_org.location::geometry), ST_X(v_org.location::geometry), p_organization_id);
        END IF;
        RETURN v_org.stop_place_id;
    END IF;

    IF NOT COALESCE(p_create, true) THEN
        RETURN NULL;
    END IF;

    SELECT sp.id INTO v_place
    FROM tracking.stop_places sp
    WHERE sp.tenant_id = p_tenant_id
      AND sp.location IS NOT NULL
      AND sp.organization_id = p_organization_id
      AND ST_DWithin(sp.location, v_org.location, 30)
    ORDER BY ST_Distance(sp.location, v_org.location)
    LIMIT 1;
    IF v_place IS NULL THEN
        INSERT INTO tracking.stop_places (tenant_id, organization_id, name, address, location)
        VALUES (p_tenant_id, p_organization_id, v_org.name, v_org.address, v_org.location)
        RETURNING tracking.stop_places.id INTO v_place;
    END IF;

    UPDATE tracking.organizations o SET stop_place_id = v_place WHERE o.id = p_organization_id;
    RETURN v_place;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_organization_stop_sync(UUID, UUID, BOOLEAN) IS
'Keeps the client''s stop on its pin and name (TRACK-045 D3); without one and p_create, links its nearest own stop within 30 m or a new one; NULL for a client without a pin';

-- ===========================================================================
-- Organization reads and writes carry stop_place_id
-- ===========================================================================

DROP FUNCTION tracking.sp_create_organization(UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, INT, DECIMAL, DECIMAL, VARCHAR);
DROP FUNCTION tracking.sp_update_organization(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, INT, DECIMAL, DECIMAL, VARCHAR);
DROP FUNCTION tracking.sp_list_organizations(UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT);
DROP FUNCTION tracking.sp_get_organization(UUID, UUID);
DROP FUNCTION tracking.fn_organization_row(UUID, UUID);

CREATE FUNCTION tracking.fn_organization_row(p_tenant_id UUID, p_organization_id UUID)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    absence_cutoff_min INT,
    latitude DECIMAL,
    longitude DECIMAL,
    address VARCHAR,
    stop_place_id UUID
) AS $$
    SELECT
        o.id,
        o.tenant_id,
        CAST(o.kind AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(o.timezone AS VARCHAR),
        o.is_active,
        o.created_at,
        o.updated_at,
        o.absence_cutoff_min,
        CAST(ST_Y(o.location::geometry) AS DECIMAL),
        CAST(ST_X(o.location::geometry) AS DECIMAL),
        CAST(o.address AS VARCHAR),
        o.stop_place_id
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
$$ LANGUAGE sql STABLE;

CREATE FUNCTION tracking.sp_create_organization(
    p_tenant_id UUID,
    p_kind VARCHAR(20),
    p_name VARCHAR(255),
    p_timezone VARCHAR(64),
    p_is_active BOOLEAN,
    p_absence_cutoff_min INT DEFAULT NULL,
    p_latitude DECIMAL DEFAULT NULL,
    p_longitude DECIMAL DEFAULT NULL,
    p_address VARCHAR(500) DEFAULT NULL,
    p_create_stop BOOLEAN DEFAULT true
)
RETURNS TABLE(
    id UUID, tenant_id UUID, kind VARCHAR, name VARCHAR, timezone VARCHAR, is_active BOOLEAN,
    created_at TIMESTAMP, updated_at TIMESTAMP, absence_cutoff_min INT,
    latitude DECIMAL, longitude DECIMAL, address VARCHAR, stop_place_id UUID
) AS $$
DECLARE
    v_id UUID;
BEGIN
    INSERT INTO tracking.organizations (tenant_id, kind, name, timezone, is_active, absence_cutoff_min, location, address)
    VALUES (
        p_tenant_id,
        COALESCE(NULLIF(TRIM(p_kind), ''), 'other'),
        TRIM(p_name),
        COALESCE(NULLIF(TRIM(p_timezone), ''), 'UTC'),
        COALESCE(p_is_active, true),
        COALESCE(p_absence_cutoff_min, 60),
        CASE WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography END,
        NULLIF(TRIM(p_address), '')
    )
    RETURNING tracking.organizations.id INTO v_id;

    PERFORM tracking.fn_organization_stop_sync(p_tenant_id, v_id, p_create_stop);
    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, v_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_organization(UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, INT, DECIMAL, DECIMAL, VARCHAR, BOOLEAN) IS
'Creates a client; with a pin and p_create_stop (default) it also gets its own stop (TRACK-045 D3)';

-- Latitude and longitude are written as sent, both or neither: the form owns the pin. A linked stop
-- always follows the client; p_create_stop only decides whether a client without one gets one.
CREATE FUNCTION tracking.sp_update_organization(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_kind VARCHAR(20),
    p_name VARCHAR(255),
    p_timezone VARCHAR(64),
    p_is_active BOOLEAN,
    p_absence_cutoff_min INT DEFAULT NULL,
    p_latitude DECIMAL DEFAULT NULL,
    p_longitude DECIMAL DEFAULT NULL,
    p_address VARCHAR(500) DEFAULT NULL,
    p_create_stop BOOLEAN DEFAULT true
)
RETURNS TABLE(
    id UUID, tenant_id UUID, kind VARCHAR, name VARCHAR, timezone VARCHAR, is_active BOOLEAN,
    created_at TIMESTAMP, updated_at TIMESTAMP, absence_cutoff_min INT,
    latitude DECIMAL, longitude DECIMAL, address VARCHAR, stop_place_id UUID
) AS $$
BEGIN
    UPDATE tracking.organizations o SET
        kind      = COALESCE(NULLIF(TRIM(p_kind), ''), o.kind),
        name      = COALESCE(NULLIF(TRIM(p_name), ''), o.name),
        timezone  = COALESCE(NULLIF(TRIM(p_timezone), ''), o.timezone),
        is_active = COALESCE(p_is_active, o.is_active),
        absence_cutoff_min = COALESCE(p_absence_cutoff_min, o.absence_cutoff_min),
        location  = CASE WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography ELSE o.location END,
        address   = COALESCE(NULLIF(TRIM(p_address), ''), o.address),
        updated_at = CURRENT_TIMESTAMP
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_organization_stop_sync(p_tenant_id, p_organization_id, p_create_stop);
    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, p_organization_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_organization(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, INT, DECIMAL, DECIMAL, VARCHAR, BOOLEAN) IS
'Edits a client; its stop moves and renames with it, and a pinned client without one gets one unless p_create_stop is false (TRACK-045 D3)';

CREATE FUNCTION tracking.sp_list_organizations(
    p_tenant_id UUID,
    p_search    VARCHAR DEFAULT NULL,
    p_kind      VARCHAR DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL,
    p_page      INT     DEFAULT 1,
    p_page_size INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID, tenant_id UUID, kind VARCHAR, name VARCHAR, timezone VARCHAR, is_active BOOLEAN,
    created_at TIMESTAMP, updated_at TIMESTAMP, absence_cutoff_min INT,
    latitude DECIMAL, longitude DECIMAL, address VARCHAR, stop_place_id UUID,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT fr.*, CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.organizations o
    CROSS JOIN LATERAL tracking.fn_organization_row(p_tenant_id, o.id) fr
    WHERE o.tenant_id = p_tenant_id
      AND (p_search IS NULL OR p_search = '' OR o.name ILIKE '%' || p_search || '%')
      AND (p_kind IS NULL OR p_kind = '' OR o.kind = p_kind)
      AND (p_is_active IS NULL OR o.is_active = p_is_active)
    -- Alphabetical, so the page boundary follows the order the reader sees; o.id breaks ties.
    ORDER BY o.name ASC, o.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_organizations(UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT) IS
'One page of a tenant''s organizations by name, with their location (TRACK-038 D2) and stop (TRACK-045 D3), optionally narrowed by a name search, kind and status; every row carries the total_count the filters match';

CREATE FUNCTION tracking.sp_get_organization(
    p_tenant_id UUID,
    p_organization_id UUID
)
RETURNS TABLE(
    id UUID, tenant_id UUID, kind VARCHAR, name VARCHAR, timezone VARCHAR, is_active BOOLEAN,
    created_at TIMESTAMP, updated_at TIMESTAMP, absence_cutoff_min INT,
    latitude DECIMAL, longitude DECIMAL, address VARCHAR, stop_place_id UUID
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, p_organization_id);
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- Route pairs call at the client's stop
-- ===========================================================================

-- As TRACK-044 left it, except the destination: the client's own stop when it has one and the form
-- sent no other point (or one within 30 m of it), so every pair of the client shares one place.
CREATE OR REPLACE FUNCTION tracking.sp_create_route_pair(
    p_tenant_id           UUID,
    p_company_id          UUID,
    p_organization_id     UUID,
    p_route_name          VARCHAR(255),
    p_vehicle_id          UUID,
    p_with_return         BOOLEAN,
    p_arrival_time        TIME,
    p_departure_time      TIME,
    p_days                SMALLINT,
    p_stops               JSONB,
    p_timezone            VARCHAR(64),
    p_return_route_name   VARCHAR(255),
    p_destination_lat     DECIMAL,
    p_destination_lng     DECIMAL,
    p_destination_address VARCHAR(500),
    p_created_by          UUID
)
RETURNS TABLE(route_id UUID, return_route_id UUID) AS $$
DECLARE
    v_org_name  VARCHAR;
    v_dest      GEOGRAPHY;
    v_dest_addr VARCHAR;
    v_dest_item JSONB;
    v_org_stop  UUID;
    v_stop_at   GEOGRAPHY;
    v_stops     JSONB := '[]'::jsonb;
    v_first     GEOGRAPHY;
    v_first_lbl VARCHAR;
    v_prev      GEOGRAPHY;
    v_point     GEOGRAPHY;
    v_metres    DOUBLE PRECISION := 0;
    v_count     INT := 0;
    v_minutes   INT;
    v_item      JSONB;
    v_out       UUID;
    v_ret       UUID;
    v_schedule  UUID;
    v_days      SMALLINT := COALESCE(p_days, 31);
BEGIN
    SELECT o.name, o.location, o.address, sp.id, sp.location
      INTO v_org_name, v_dest, v_dest_addr, v_org_stop, v_stop_at
    FROM tracking.organizations o
    LEFT JOIN tracking.stop_places sp ON sp.id = o.stop_place_id AND sp.location IS NOT NULL
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF v_org_name IS NULL THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF p_destination_lat IS NOT NULL AND p_destination_lng IS NOT NULL THEN
        v_dest := ST_SetSRID(ST_MakePoint(p_destination_lng, p_destination_lat), 4326)::geography;
    END IF;
    IF v_dest IS NULL THEN
        RAISE EXCEPTION 'route.destination-required' USING ERRCODE = 'P0001';
    END IF;
    IF COALESCE(p_with_return, true) AND p_departure_time IS NULL THEN
        RAISE EXCEPTION 'route.departure-required' USING ERRCODE = 'P0001';
    END IF;
    v_dest_addr := COALESCE(NULLIF(TRIM(p_destination_address), ''), v_dest_addr, v_org_name);
    IF v_org_stop IS NOT NULL AND ST_DWithin(v_stop_at, v_dest, 30) THEN
        v_dest := v_stop_at;
        v_dest_item := jsonb_build_object('stop_place_id', v_org_stop);
    ELSE
        v_dest_item := jsonb_build_object(
            'name', v_org_name, 'address', v_dest_addr,
            'latitude', ST_Y(v_dest::geometry), 'longitude', ST_X(v_dest::geometry));
    END IF;

    -- The pickups in order, without the editor's own sequence (the list order is the order). A body
    -- without stops sends JSON null, which is not SQL NULL: anything but an array is no stops.
    FOR v_item IN SELECT e FROM jsonb_array_elements(
        CASE WHEN jsonb_typeof(p_stops) = 'array' THEN p_stops ELSE '[]'::jsonb END) e LOOP
        v_point := tracking.fn_stop_item_point(p_tenant_id, v_item);
        IF v_point IS NULL THEN
            RAISE EXCEPTION 'stop-place.invalid' USING ERRCODE = 'P0001';
        END IF;
        IF v_first IS NULL THEN
            v_first := v_point;
            v_first_lbl := COALESCE(NULLIF(TRIM(v_item->>'address'), ''), NULLIF(TRIM(v_item->>'name'), ''), (
                SELECT COALESCE(sp.address, sp.name) FROM tracking.stop_places sp
                WHERE sp.id = NULLIF(v_item->>'stop_place_id', '')::UUID));
        ELSE
            v_metres := v_metres + ST_Distance(v_prev, v_point);
        END IF;
        v_prev := v_point;
        v_count := v_count + 1;
        v_stops := v_stops || jsonb_build_array(v_item - 'sequence');
    END LOOP;
    IF v_prev IS NOT NULL THEN
        v_metres := v_metres + ST_Distance(v_prev, v_dest);
    END IF;
    v_minutes := GREATEST(5, CAST(CEIL((v_metres * 1.3 / 1000 / 25 * 60 + v_count) / 5.0) * 5 AS INT));

    SELECT r.id INTO v_out FROM tracking.sp_create_route(
        p_tenant_id, p_company_id, p_route_name,
        COALESCE(v_first_lbl, v_dest_addr), v_dest_addr, 'outbound',
        p_vehicle_id, NULL, p_timezone, NULL,
        CAST(ST_Y(v_first::geometry) AS DECIMAL), CAST(ST_X(v_first::geometry) AS DECIMAL),
        CAST(ST_Y(v_dest::geometry) AS DECIMAL), CAST(ST_X(v_dest::geometry) AS DECIMAL),
        v_minutes, NULL) r;
    UPDATE tracking.routes r SET organization_id = p_organization_id WHERE r.id = v_out;
    PERFORM tracking.sp_create_route_version(p_tenant_id, v_out, tracking.fn_route_today(v_out),
        v_stops || jsonb_build_array(v_dest_item), p_created_by);
    IF p_arrival_time IS NOT NULL THEN
        SELECT s.id INTO v_schedule FROM tracking.sp_create_route_schedule(p_tenant_id, v_out, v_days,
            CAST(p_arrival_time - make_interval(mins => v_minutes) AS TIME), tracking.fn_route_today(v_out), NULL, NULL) s;
        UPDATE tracking.route_schedules s SET departure_auto_min = v_minutes WHERE s.id = v_schedule;
    END IF;

    IF COALESCE(p_with_return, true) THEN
        SELECT r.id INTO v_ret FROM tracking.sp_create_route(
            p_tenant_id, p_company_id,
            COALESCE(NULLIF(TRIM(p_return_route_name), ''), TRIM(p_route_name) || ' (vuelta)'),
            v_dest_addr, COALESCE(v_first_lbl, v_dest_addr), 'inbound',
            p_vehicle_id, NULL, p_timezone, NULL,
            CAST(ST_Y(v_dest::geometry) AS DECIMAL), CAST(ST_X(v_dest::geometry) AS DECIMAL),
            CAST(ST_Y(v_first::geometry) AS DECIMAL), CAST(ST_X(v_first::geometry) AS DECIMAL),
            v_minutes, NULL) r;
        UPDATE tracking.routes r SET organization_id = p_organization_id, paired_route_id = v_out WHERE r.id = v_ret;
        UPDATE tracking.routes r SET paired_route_id = v_ret WHERE r.id = v_out;
        PERFORM tracking.sp_create_route_version(p_tenant_id, v_ret, tracking.fn_route_today(v_ret),
            jsonb_build_array(v_dest_item) || COALESCE((
                SELECT jsonb_agg(e.value ORDER BY e.ordinality DESC)
                FROM jsonb_array_elements(v_stops) WITH ORDINALITY AS e(value, ordinality)
            ), '[]'::jsonb),
            p_created_by);
        PERFORM tracking.sp_create_route_schedule(p_tenant_id, v_ret, v_days, p_departure_time,
            tracking.fn_route_today(v_ret), NULL, NULL);
    END IF;

    RETURN QUERY SELECT v_out, v_ret;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_route_pair(UUID, UUID, UUID, VARCHAR, UUID, BOOLEAN, TIME, TIME, SMALLINT, JSONB, VARCHAR, VARCHAR, DECIMAL, DECIMAL, VARCHAR, UUID) IS
'Creates a destination''s outbound route (stops, possibly none, then the destination) and, with p_with_return, its return (reversed), same vehicle, linked both ways, each with its schedule, in one transaction (TRACK-038 D1, D2); the outbound departure follows its stops (TRACK-044 D2); the destination is the client''s own stop when it has one (TRACK-045 D3); raises tracking.organization.not-found, tracking.route.destination-required, tracking.route.departure-required';

-- ===========================================================================
-- Backfill: every pinned client gets its stop, and its routes' copies of the gate point at it
-- ===========================================================================

DO $$
DECLARE
    v_org RECORD;
    v_place UUID;
BEGIN
    FOR v_org IN
        SELECT o.id, o.tenant_id, o.name, o.location FROM tracking.organizations o
        WHERE o.location IS NOT NULL AND o.stop_place_id IS NULL
    LOOP
        v_place := tracking.fn_organization_stop_sync(v_org.tenant_id, v_org.id, true);

        -- The destination copies earlier pairs made (named after the client, within 30 m, on its routes)
        -- become the client's stop; a version that already calls there keeps its own row.
        UPDATE tracking.route_version_stops vs SET stop_place_id = v_place, updated_at = CURRENT_TIMESTAMP
        FROM tracking.route_versions rv, tracking.routes r, tracking.stop_places sp
        WHERE rv.id = vs.version_id
          AND r.id = rv.route_id AND r.organization_id = v_org.id
          AND sp.id = vs.stop_place_id AND sp.id <> v_place
          AND sp.name = v_org.name
          AND sp.location IS NOT NULL AND ST_DWithin(sp.location, v_org.location, 30)
          AND NOT EXISTS (
              SELECT 1 FROM tracking.route_version_stops x
              WHERE x.version_id = vs.version_id AND x.stop_place_id = v_place);
    END LOOP;
END;
$$;
