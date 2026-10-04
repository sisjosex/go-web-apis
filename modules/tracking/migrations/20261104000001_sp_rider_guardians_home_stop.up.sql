-- TRACK-037: a rider's guardians come with the rider, and the rider's home is the default stop.
--
-- D2  The rider form carries its guardians. The accounts are granted by tenancy first (main database,
--     not this one), then fn_rider_guardians_link writes every link in the statement that creates the
--     rider, so a rider never exists without the guardians the form named.
-- D3  A guardian without an email is a contact row (user_id NULL): the Family tab lists it beside
--     the accounts.
-- D1  An assignment created with no pickup (outbound) or no drop-off (inbound) for a rider with a
--     home pin calls at the home: a place of the tenant within 30 m is reused, otherwise one is made,
--     and it is inserted where it lengthens the route least (straight-line PostGIS distance). Versions
--     in force from the assignment's first day on get it; one that started earlier is split there,
--     so a day already run never changes.

-- ===========================================================================
-- D2 / D3 guardians
-- ===========================================================================

-- p_guardians: [{user_id, name, email, phone}] for granted accounts, [{name, phone}] for a contact.
-- The first refused item raises tracking.rider.guardian-invalid with DETAIL index=<n>.
CREATE FUNCTION tracking.fn_rider_guardians_link(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_guardians JSONB
)
RETURNS INT AS $$
DECLARE
    v_item  RECORD;
    v_count INT := 0;
BEGIN
    FOR v_item IN
        SELECT e.value, e.ordinality - 1 AS idx
        FROM jsonb_array_elements(COALESCE(p_guardians, '[]'::jsonb)) WITH ORDINALITY AS e(value, ordinality)
    LOOP
        IF NULLIF(v_item.value->>'user_id', '') IS NOT NULL THEN
            PERFORM tracking.sp_rider_guardian_add(
                p_tenant_id, p_rider_id, (v_item.value->>'user_id')::UUID,
                v_item.value->>'name', v_item.value->>'email', v_item.value->>'phone', NULL);
        ELSIF NULLIF(TRIM(v_item.value->>'name'), '') IS NOT NULL AND NULLIF(TRIM(v_item.value->>'phone'), '') IS NOT NULL THEN
            INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, is_primary)
            VALUES (
                p_rider_id, 'guardian', TRIM(v_item.value->>'name'), TRIM(v_item.value->>'phone'),
                NOT EXISTS (
                    SELECT 1 FROM tracking.rider_contacts c
                    WHERE c.rider_id = p_rider_id AND c.relation = 'guardian' AND c.is_primary
                )
            );
        ELSE
            RAISE EXCEPTION 'rider.guardian-invalid' USING ERRCODE = 'P0001', DETAIL = 'index=' || v_item.idx;
        END IF;
        v_count := v_count + 1;
    END LOOP;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_rider_guardians_link(UUID, UUID, JSONB) IS
'Links a new rider''s guardians: granted accounts {user_id, name, email, phone} and contacts {name, phone}; raises tracking.rider.guardian-invalid with DETAIL index=<n> (TRACK-037 D2, D3)';

-- The Family tab lists contacts (user_id NULL) after the accounts.
CREATE OR REPLACE FUNCTION tracking.sp_list_rider_guardians(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_scope_user_id UUID
)
RETURNS TABLE(
    user_id UUID,
    email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    is_primary BOOLEAN
)
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id);

    RETURN QUERY
    SELECT g.user_id, g.email, g.first_name, g.last_name, g.phone, g.is_primary FROM (
        SELECT DISTINCT ON (COALESCE(rc.user_id, rc.id))
            rc.user_id,
            CAST(COALESCE(u.email, rc.email) AS VARCHAR) AS email,
            CAST(COALESCE(u.first_name, rc.name) AS VARCHAR) AS first_name,
            CAST(u.last_name AS VARCHAR) AS last_name,
            CAST(COALESCE(rc.phone, u.phone) AS VARCHAR) AS phone,
            rc.is_primary
        FROM tracking.rider_contacts rc
        -- auth.users sits beside us on a shared database; elsewhere the contact's own copy answers.
        LEFT JOIN auth.users u ON u.id = rc.user_id AND u.deleted_at IS NULL
        WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian'
          AND (rc.user_id IS NOT NULL OR (rc.name IS NOT NULL AND rc.phone IS NOT NULL))
        ORDER BY COALESCE(rc.user_id, rc.id), rc.is_primary DESC
    ) g
    ORDER BY (g.user_id IS NULL), g.is_primary DESC, g.first_name, g.last_name;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_rider_guardians(UUID, UUID, UUID) IS
'The guardians of a rider: accounts first, then contacts without one (user_id NULL, TRACK-037 D3); raises tracking.rider.not-found outside the tenant or the caller''s organization scope';

-- ===========================================================================
-- D1 home stop
-- ===========================================================================

-- Returns the home's stop place, now on every version of the route in force from p_on, or NULL when
-- the rider has no home pin.
CREATE FUNCTION tracking.fn_route_home_stop(
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
