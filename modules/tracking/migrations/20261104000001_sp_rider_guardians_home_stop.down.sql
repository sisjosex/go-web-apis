-- TRACK-037 rollback: guardians linked one by one again, no home stop, accounts-only Family tab.

DROP FUNCTION tracking.fn_rider_guardians_link(UUID, UUID, JSONB);
DROP FUNCTION tracking.fn_route_home_stop(UUID, UUID, UUID, DATE);

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
    SELECT g.* FROM (
        SELECT DISTINCT ON (rc.user_id)
            rc.user_id,
            CAST(COALESCE(u.email, rc.email) AS VARCHAR) AS email,
            CAST(COALESCE(u.first_name, rc.name) AS VARCHAR) AS first_name,
            CAST(u.last_name AS VARCHAR) AS last_name,
            CAST(COALESCE(rc.phone, u.phone) AS VARCHAR) AS phone,
            rc.is_primary
        FROM tracking.rider_contacts rc
        -- auth.users sits beside us on a shared database; elsewhere the contact's own copy answers.
        LEFT JOIN auth.users u ON u.id = rc.user_id AND u.deleted_at IS NULL
        WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian' AND rc.user_id IS NOT NULL
        ORDER BY rc.user_id, rc.is_primary DESC
    ) g
    ORDER BY g.is_primary DESC, g.first_name, g.last_name;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_rider_guardians(UUID, UUID, UUID) IS
'The guardian accounts of a rider (D3); raises tracking.rider.not-found outside the tenant or the caller''s organization scope';

CREATE OR REPLACE FUNCTION tracking.fn_assignment_put(
    p_tenant_id UUID,
    p_id UUID,
    p_item JSONB
)
RETURNS UUID AS $$
DECLARE
    v_old     tracking.rider_route_assignments%ROWTYPE;
    v_id      UUID;
    v_rider   UUID;
    v_route   UUID;
    v_days    SMALLINT;
    v_pickup  UUID;
    v_dropoff UUID;
    v_from    DATE;
    v_until   DATE;
    v_on      DATE;
    v_version UUID;
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

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = v_route AND tc.tenant_id = p_tenant_id
    ) THEN
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
    IF v_pickup IS NOT NULL OR v_dropoff IS NOT NULL THEN
        v_on := GREATEST(v_from, tracking.fn_route_today(v_route));
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
        INNER JOIN tracking.routes nr ON nr.id = v_route
        WHERE a.rider_id = v_rider
          AND a.id IS DISTINCT FROM p_id
          AND ar.direction = nr.direction
          AND daterange(a.valid_from, a.valid_until, '[]') && daterange(v_from, v_until, '[]')
          AND (a.days_of_week & v_days) <> 0
    ) THEN
        RAISE EXCEPTION 'assignment.overlap' USING ERRCODE = 'P0001';
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

    PERFORM tracking.fn_route_changed(v_route, GREATEST(v_from, tracking.fn_route_today(v_route)), v_until);
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_assignment_put(UUID, UUID, JSONB) IS
'Creates (p_id NULL) or updates one assignment after every check the endpoints share, and writes route.changed for the days it covers and covered (TRACK-009 D4)';
