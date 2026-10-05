-- TRACK-044 down: the functions as TRACK-029/037/038/043 left them, then the new objects go.

DROP FUNCTION tracking.sp_delete_rider_place(UUID, UUID, UUID, UUID);
DROP FUNCTION tracking.sp_update_rider_place(UUID, UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, BOOLEAN, UUID);
DROP FUNCTION tracking.sp_create_rider_place(UUID, UUID, VARCHAR, VARCHAR, DECIMAL, DECIMAL, BOOLEAN, UUID);
DROP FUNCTION tracking.sp_list_rider_places(UUID, UUID, UUID);
DROP FUNCTION tracking.fn_rider_place_rows(UUID, UUID);

CREATE OR REPLACE FUNCTION tracking.sp_create_rider(
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
    p_group_label VARCHAR(100) DEFAULT NULL,
    p_service_legs VARCHAR(10) DEFAULT NULL,
    p_stop_place_id UUID DEFAULT NULL
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
    group_label VARCHAR,
    service_legs VARCHAR,
    stop_place_id UUID,
    stop_place_name VARCHAR
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
        identification_number, phone, email, status, home_location, notes, group_label,
        address, service_legs, stop_place_id
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
        NULLIF(TRIM(p_group_label), ''),
        NULLIF(TRIM(p_address), ''),
        COALESCE(NULLIF(TRIM(p_service_legs), ''), 'both'),
        tracking.fn_rider_stop_place(p_tenant_id, p_stop_place_id)
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
        CAST(r.address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(r.group_label AS VARCHAR),
        CAST(r.service_legs AS VARCHAR),
        r.stop_place_id,
        CAST(sp.name AS VARCHAR)
    FROM tracking.riders r
    LEFT JOIN tracking.stop_places sp ON sp.id = r.stop_place_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = v_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_update_rider(
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
    p_group_label VARCHAR(100) DEFAULT NULL,
    p_service_legs VARCHAR(10) DEFAULT NULL,
    p_stop_place_id UUID DEFAULT NULL
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
    group_label VARCHAR,
    service_legs VARCHAR,
    stop_place_id UUID,
    stop_place_name VARCHAR
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
        -- TRACK-043: NULL keeps each of these; '' clears the address.
        address = CASE WHEN p_address IS NULL THEN r.address ELSE NULLIF(TRIM(p_address), '') END,
        service_legs = COALESCE(NULLIF(TRIM(p_service_legs), ''), r.service_legs),
        stop_place_id = COALESCE(tracking.fn_rider_stop_place(p_tenant_id, p_stop_place_id), r.stop_place_id),
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
        CAST(r.address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(r.group_label AS VARCHAR),
        CAST(r.service_legs AS VARCHAR),
        r.stop_place_id,
        CAST(sp.name AS VARCHAR)
    FROM tracking.riders r
    LEFT JOIN tracking.stop_places sp ON sp.id = r.stop_place_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;


DROP FUNCTION tracking.fn_rider_home_place(UUID);
DROP FUNCTION tracking.fn_rider_place_mirror(UUID);
DROP FUNCTION tracking.fn_rider_place_guard(UUID, UUID, UUID);
DROP TABLE tracking.rider_places;

-- fn_assignment_put as TRACK-009 wrote it, plus D1: a new assignment with no stop on its route's
-- leg (pickup outbound, drop-off inbound) calls at the home, once every check has passed.
CREATE OR REPLACE FUNCTION tracking.fn_assignment_put(
    p_tenant_id UUID,
    p_id UUID,
    p_item JSONB
)
RETURNS UUID AS $$
DECLARE
    v_old       tracking.rider_route_assignments%ROWTYPE;
    v_id        UUID;
    v_rider     UUID;
    v_route     UUID;
    v_days      SMALLINT;
    v_pickup    UUID;
    v_dropoff   UUID;
    v_from      DATE;
    v_until     DATE;
    v_on        DATE;
    v_version   UUID;
    v_direction VARCHAR;
BEGIN
    IF p_id IS NOT NULL THEN
        SELECT a.* INTO v_old
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.riders rd ON rd.id = a.rider_id
        INNER JOIN tracking.organizations o ON o.id = rd.organization_id
        WHERE a.id = p_id AND o.tenant_id = p_tenant_id;
        IF v_old.id IS NULL THEN
            RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
        END IF;
        v_rider := v_old.rider_id;
    ELSE
        v_rider := (p_item->>'rider_id')::UUID;
    END IF;

    v_route   := CASE WHEN p_item ? 'route_id' THEN (p_item->>'route_id')::UUID ELSE v_old.route_id END;
    v_days    := CASE WHEN p_item ? 'days_of_week' THEN (p_item->>'days_of_week')::SMALLINT ELSE v_old.days_of_week END;
    v_pickup  := CASE WHEN p_item ? 'pickup_stop_place_id' THEN (p_item->>'pickup_stop_place_id')::UUID ELSE v_old.pickup_stop_place_id END;
    v_dropoff := CASE WHEN p_item ? 'dropoff_stop_place_id' THEN (p_item->>'dropoff_stop_place_id')::UUID ELSE v_old.dropoff_stop_place_id END;
    v_from    := CASE WHEN p_item ? 'valid_from' THEN (p_item->>'valid_from')::DATE ELSE v_old.valid_from END;
    v_until   := CASE WHEN p_item ? 'valid_until' THEN (p_item->>'valid_until')::DATE ELSE v_old.valid_until END;

    -- The rider row is the lock (D4): every write for one rider queues here, so two concurrent
    -- writes cannot both pass the overlap check — even when the rider has no assignment yet.
    PERFORM 1
    FROM tracking.riders rd
    INNER JOIN tracking.organizations o ON o.id = rd.organization_id
    WHERE rd.id = v_rider AND o.tenant_id = p_tenant_id
    FOR UPDATE OF rd;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT r.direction INTO v_direction
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = v_route AND tc.tenant_id = p_tenant_id;
    IF v_direction IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_days IS NULL OR v_days NOT BETWEEN 1 AND 127 THEN
        RAISE EXCEPTION 'assignment.days-of-week' USING ERRCODE = 'P0001';
    END IF;
    IF v_from IS NULL OR v_until < v_from THEN
        RAISE EXCEPTION 'assignment.range' USING ERRCODE = 'P0001';
    END IF;

    -- A stop must be one the route calls at in the version in force when the assignment first rides:
    -- the route's today, or the assignment's start when that is later.
    v_on := GREATEST(v_from, tracking.fn_route_today(v_route));
    IF v_pickup IS NOT NULL OR v_dropoff IS NOT NULL THEN
        SELECT rv.id INTO v_version
        FROM tracking.route_versions rv
        WHERE rv.route_id = v_route
          AND v_on >= rv.effective_from
          AND (rv.effective_to IS NULL OR v_on <= rv.effective_to);

        IF EXISTS (
            SELECT 1 FROM unnest(ARRAY[v_pickup, v_dropoff]) s(stop_place_id)
            WHERE s.stop_place_id IS NOT NULL
              AND NOT EXISTS (
                  SELECT 1 FROM tracking.route_version_stops vs
                  WHERE vs.version_id = v_version AND vs.stop_place_id = s.stop_place_id
              )
        ) THEN
            RAISE EXCEPTION 'assignment.stop-not-on-route' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Overlap (D4): the same rider, a route of the same direction (D2), dates that meet and a weekday
    -- in common. The rider's rows are few and indexed by rider_id.
    IF EXISTS (
        SELECT 1
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes ar ON ar.id = a.route_id
        WHERE a.rider_id = v_rider
          AND a.id IS DISTINCT FROM p_id
          AND ar.direction = v_direction
          AND daterange(a.valid_from, a.valid_until, '[]') && daterange(v_from, v_until, '[]')
          AND (a.days_of_week & v_days) <> 0
    ) THEN
        RAISE EXCEPTION 'assignment.overlap' USING ERRCODE = 'P0001';
    END IF;

    -- TRACK-037 D1: the home is the stop of the leg the caller left empty.
    IF p_id IS NULL THEN
        IF v_direction = 'outbound' AND v_pickup IS NULL THEN
            v_pickup := tracking.fn_route_home_stop(p_tenant_id, v_route, v_rider, v_on);
        ELSIF v_direction = 'inbound' AND v_dropoff IS NULL THEN
            v_dropoff := tracking.fn_route_home_stop(p_tenant_id, v_route, v_rider, v_on);
        END IF;
    END IF;

    IF p_id IS NULL THEN
        INSERT INTO tracking.rider_route_assignments (
            rider_id, route_id, days_of_week, pickup_stop_place_id, dropoff_stop_place_id, valid_from, valid_until
        )
        VALUES (v_rider, v_route, v_days, v_pickup, v_dropoff, v_from, v_until)
        RETURNING tracking.rider_route_assignments.id INTO v_id;
    ELSE
        UPDATE tracking.rider_route_assignments a
        SET route_id = v_route,
            days_of_week = v_days,
            pickup_stop_place_id = v_pickup,
            dropoff_stop_place_id = v_dropoff,
            valid_from = v_from,
            valid_until = v_until,
            updated_at = CURRENT_TIMESTAMP
        WHERE a.id = p_id;
        v_id := p_id;

        -- The days the row used to cover are re-planned too, on the route it used to be on.
        PERFORM tracking.fn_route_changed(
            v_old.route_id, GREATEST(v_old.valid_from, tracking.fn_route_today(v_old.route_id)), v_old.valid_until
        );
    END IF;

    PERFORM tracking.fn_route_changed(v_route, v_on, v_until);
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_assignment_put(UUID, UUID, JSONB) IS
'Creates (p_id NULL) or updates one assignment after every check the endpoints share, putting a new assignment''s empty leg stop at the rider''s home (TRACK-037 D1), and writes route.changed for the days it covers and covered (TRACK-009 D4)';

-- Removing one leg of a pair removes its twin: the same rider, days and start on the other route.
CREATE OR REPLACE FUNCTION tracking.sp_delete_rider_route_assignment(
    p_tenant_id UUID,
    p_id UUID
)
RETURNS VOID AS $$
DECLARE
    v_old  tracking.rider_route_assignments%ROWTYPE;
    v_twin tracking.rider_route_assignments%ROWTYPE;
BEGIN
    DELETE FROM tracking.rider_route_assignments a
    USING tracking.riders rd, tracking.organizations o
    WHERE a.id = p_id
      AND rd.id = a.rider_id
      AND o.id = rd.organization_id
      AND o.tenant_id = p_tenant_id
    RETURNING a.* INTO v_old;

    IF v_old.id IS NULL THEN
        RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_route_changed(
        v_old.route_id, GREATEST(v_old.valid_from, tracking.fn_route_today(v_old.route_id)), v_old.valid_until
    );

    DELETE FROM tracking.rider_route_assignments a
    USING tracking.routes r
    WHERE r.id = v_old.route_id
      AND a.route_id = r.paired_route_id
      AND a.rider_id = v_old.rider_id
      AND a.days_of_week = v_old.days_of_week
      AND a.valid_from = v_old.valid_from
    RETURNING a.* INTO v_twin;
    IF v_twin.id IS NOT NULL THEN
        PERFORM tracking.fn_route_changed(
            v_twin.route_id, GREATEST(v_twin.valid_from, tracking.fn_route_today(v_twin.route_id)), v_twin.valid_until
        );
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_rider_route_assignment(UUID, UUID) IS
'Removes one assignment, and its twin on the paired route (TRACK-038 D1), writing route.changed for the days they covered from the route''s today (TRACK-009)';

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
    -- TRACK-043 D3: a shared stop the import named is the rider's stop instead of the home.
    SELECT COALESCE(sp.location, rd.home_location), CAST(rd.first_name || ' ' || rd.last_name AS VARCHAR), sp.id
      INTO v_home, v_name, v_place
    FROM tracking.riders rd
    LEFT JOIN tracking.stop_places sp ON sp.id = rd.stop_place_id AND sp.location IS NOT NULL
    WHERE rd.id = p_rider_id;
    IF v_home IS NULL THEN
        RETURN NULL;
    END IF;

    IF v_place IS NULL THEN
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
'Puts the rider''s home (a place within 30 m, or a new one) on every version of the route in force from p_on, at the cheapest straight-line position, splitting a version that started earlier; NULL when the rider has no home pin (TRACK-037 D1); a rider with a shared stop (TRACK-043 D3) uses it instead';


DROP FUNCTION tracking.fn_route_prune_home_stop(UUID, UUID, UUID, UUID, DATE);
DROP FUNCTION tracking.fn_route_version_split(UUID, DATE);

-- p_stops are the pickups in the outbound order; the destination is appended as the last stop and
-- leads the return, whose stops run in reverse. Until the line is traced (it then writes the real
-- duration), the outbound leaves an estimate before p_arrival_time: straight-line kilometres × 1.3
-- at 25 km/h plus a minute per stop, rounded up to 5 minutes.
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
    v_days      SMALLINT := COALESCE(p_days, 31);
BEGIN
    SELECT o.name, o.location, o.address INTO v_org_name, v_dest, v_dest_addr
    FROM tracking.organizations o
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
    v_dest_item := jsonb_build_object(
        'name', v_org_name, 'address', v_dest_addr,
        'latitude', ST_Y(v_dest::geometry), 'longitude', ST_X(v_dest::geometry));

    -- The pickups in order, without the editor's own sequence (the list order is the order).
    FOR v_item IN SELECT e FROM jsonb_array_elements(COALESCE(p_stops, '[]'::jsonb)) e LOOP
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
        CAST(ST_Y(COALESCE(v_first, v_dest)::geometry) AS DECIMAL), CAST(ST_X(COALESCE(v_first, v_dest)::geometry) AS DECIMAL),
        CAST(ST_Y(v_dest::geometry) AS DECIMAL), CAST(ST_X(v_dest::geometry) AS DECIMAL),
        v_minutes, NULL) r;
    UPDATE tracking.routes r SET organization_id = p_organization_id WHERE r.id = v_out;
    PERFORM tracking.sp_create_route_version(p_tenant_id, v_out, tracking.fn_route_today(v_out),
        v_stops || jsonb_build_array(v_dest_item), p_created_by);
    IF p_arrival_time IS NOT NULL THEN
        PERFORM tracking.sp_create_route_schedule(p_tenant_id, v_out, v_days,
            CAST(p_arrival_time - make_interval(mins => v_minutes) AS TIME), tracking.fn_route_today(v_out), NULL, NULL);
    END IF;

    IF COALESCE(p_with_return, true) THEN
        SELECT r.id INTO v_ret FROM tracking.sp_create_route(
            p_tenant_id, p_company_id,
            COALESCE(NULLIF(TRIM(p_return_route_name), ''), TRIM(p_route_name) || ' (vuelta)'),
            v_dest_addr, COALESCE(v_first_lbl, v_dest_addr), 'inbound',
            p_vehicle_id, NULL, p_timezone, NULL,
            CAST(ST_Y(v_dest::geometry) AS DECIMAL), CAST(ST_X(v_dest::geometry) AS DECIMAL),
            CAST(ST_Y(COALESCE(v_first, v_dest)::geometry) AS DECIMAL), CAST(ST_X(COALESCE(v_first, v_dest)::geometry) AS DECIMAL),
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
'Creates a destination''s outbound route (stops, then the destination) and, with p_with_return, its return (reversed), same vehicle, linked both ways, each with its schedule, in one transaction (TRACK-038 D1, D2); raises tracking.organization.not-found, tracking.route.destination-required, tracking.route.departure-required';

CREATE OR REPLACE FUNCTION tracking.sp_update_route_schedule(
    p_tenant_id UUID,
    p_schedule_id UUID,
    p_days_of_week SMALLINT,
    p_start_time TIME,
    p_valid_from DATE,
    p_valid_until DATE,
    p_calendar_id UUID,
    p_clear_valid_until BOOLEAN DEFAULT FALSE,
    p_clear_calendar BOOLEAN DEFAULT FALSE
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    days_of_week SMALLINT,
    start_time VARCHAR,
    valid_from DATE,
    valid_until DATE,
    calendar_id UUID,
    calendar_name VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_route_id   UUID;
    v_start      TIME;
    v_from       DATE;
    v_until      DATE;
    v_calendar   UUID;
    v_old_from   DATE;
    v_old_until  DATE;
BEGIN
    SELECT s.route_id, s.valid_from, s.valid_until
      INTO v_route_id, v_old_from, v_old_until
    FROM tracking.route_schedules s
    INNER JOIN tracking.routes r ON r.id = s.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE s.id = p_schedule_id AND tc.tenant_id = p_tenant_id;

    IF v_route_id IS NULL THEN
        RAISE EXCEPTION 'schedule.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT
        COALESCE(p_start_time, s.start_time),
        COALESCE(p_valid_from, s.valid_from),
        CASE WHEN p_clear_valid_until THEN NULL ELSE COALESCE(p_valid_until, s.valid_until) END,
        CASE WHEN p_clear_calendar THEN NULL ELSE COALESCE(p_calendar_id, s.calendar_id) END
      INTO v_start, v_from, v_until, v_calendar
    FROM tracking.route_schedules s
    WHERE s.id = p_schedule_id;

    PERFORM tracking.fn_route_schedule_guard(
        p_tenant_id, v_route_id, v_start, v_from, v_until, v_calendar, p_schedule_id);

    BEGIN
        UPDATE tracking.route_schedules s SET
            days_of_week = COALESCE(p_days_of_week, s.days_of_week),
            start_time   = v_start,
            valid_from   = v_from,
            valid_until  = v_until,
            calendar_id  = v_calendar,
            updated_at   = CURRENT_TIMESTAMP
        WHERE s.id = p_schedule_id;
    EXCEPTION WHEN exclusion_violation THEN
        RAISE EXCEPTION 'schedule.overlap' USING ERRCODE = 'P0001';
    END;

    PERFORM tracking.fn_route_changed(
        v_route_id,
        LEAST(v_old_from, v_from),
        CASE WHEN v_old_until IS NULL OR v_until IS NULL THEN NULL ELSE GREATEST(v_old_until, v_until) END);

    RETURN QUERY
    SELECT r.id, r.route_id, r.days_of_week, r.start_time, r.valid_from, r.valid_until,
           r.calendar_id, r.calendar_name, r.created_at, r.updated_at
    FROM tracking.fn_route_schedule_row(p_tenant_id, p_schedule_id) r;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID, BOOLEAN, BOOLEAN) IS
'Edits a schedule in place; a NULL argument keeps the stored value, p_clear_valid_until and p_clear_calendar set those to NULL (TRACK-029 D4), and the route.changed row covers the old validity and the new one';


DROP FUNCTION tracking.fn_route_recompute_departure(UUID, UUID, DATE);
DROP FUNCTION tracking.fn_route_version_estimate(UUID, UUID);
ALTER TABLE tracking.route_schedules DROP COLUMN departure_auto_min;
