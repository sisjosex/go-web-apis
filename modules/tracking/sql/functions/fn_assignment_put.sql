CREATE OR REPLACE FUNCTION tracking.fn_assignment_put(p_tenant_id uuid, p_id uuid, p_item jsonb)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_old          tracking.rider_route_assignments%ROWTYPE;
    v_id           UUID;
    v_rider        UUID;
    v_route        UUID;
    v_days         SMALLINT;
    v_pickup       UUID;
    v_dropoff      UUID;
    v_pickup_home  BOOLEAN := COALESCE(p_item->>'pickup_stop_place_id' = 'home', false);
    v_dropoff_home BOOLEAN := COALESCE(p_item->>'dropoff_stop_place_id' = 'home', false);
    v_from         DATE;
    v_until        DATE;
    v_on           DATE;
    v_old_on       DATE;
    v_recheck      BOOLEAN;
    v_version      UUID;
    v_direction    VARCHAR;
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
    v_pickup  := CASE WHEN v_pickup_home THEN NULL
                      WHEN p_item ? 'pickup_stop_place_id' THEN (p_item->>'pickup_stop_place_id')::UUID
                      ELSE v_old.pickup_stop_place_id END;
    v_dropoff := CASE WHEN v_dropoff_home THEN NULL
                      WHEN p_item ? 'dropoff_stop_place_id' THEN (p_item->>'dropoff_stop_place_id')::UUID
                      ELSE v_old.dropoff_stop_place_id END;
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
    -- the route's today, or the assignment's start when that is later. An update checks the stops it
    -- sends, and every stop when it moves the route or the start (TRACK-044: a stored home stop was put
    -- there by fn_route_home_stop and is not re-judged by an edit of the end date).
    v_on := GREATEST(v_from, tracking.fn_route_today(v_route));
    v_recheck := p_id IS NULL OR v_route <> v_old.route_id OR v_from <> v_old.valid_from;
    IF v_pickup IS NOT NULL OR v_dropoff IS NOT NULL THEN
        SELECT rv.id INTO v_version
        FROM tracking.route_versions rv
        WHERE rv.route_id = v_route
          AND v_on >= rv.effective_from
          AND (rv.effective_to IS NULL OR v_on <= rv.effective_to);

        IF EXISTS (
            SELECT 1 FROM (VALUES
                (v_pickup, v_recheck OR p_item ? 'pickup_stop_place_id'),
                (v_dropoff, v_recheck OR p_item ? 'dropoff_stop_place_id')) s(stop_place_id, checked)
            WHERE s.stop_place_id IS NOT NULL
              AND s.checked
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

    -- TRACK-037 D1: the home is the stop of the leg a new assignment left empty, and of any leg sent as
    -- "home" (TRACK-044). A rider with no home and no shared stop cannot be placed (D3).
    IF v_pickup_home OR (p_id IS NULL AND v_direction = 'outbound' AND v_pickup IS NULL) THEN
        v_pickup := tracking.fn_route_home_stop(p_tenant_id, v_route, v_rider, v_on);
        IF v_pickup IS NULL THEN
            RAISE EXCEPTION 'rider.location-required' USING ERRCODE = 'P0001';
        END IF;
    END IF;
    IF v_dropoff_home OR (p_id IS NULL AND v_direction = 'inbound' AND v_dropoff IS NULL) THEN
        v_dropoff := tracking.fn_route_home_stop(p_tenant_id, v_route, v_rider, v_on);
        IF v_dropoff IS NULL THEN
            RAISE EXCEPTION 'rider.location-required' USING ERRCODE = 'P0001';
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

        -- D5: a home stop this row no longer calls at leaves the route it was on.
        v_old_on := GREATEST(v_old.valid_from, tracking.fn_route_today(v_old.route_id));
        IF v_old.route_id <> v_route OR v_old.pickup_stop_place_id IS DISTINCT FROM v_pickup THEN
            PERFORM tracking.fn_route_prune_home_stop(p_tenant_id, v_old.route_id, v_rider, v_old.pickup_stop_place_id, v_old_on);
        END IF;
        IF v_old.route_id <> v_route OR v_old.dropoff_stop_place_id IS DISTINCT FROM v_dropoff THEN
            PERFORM tracking.fn_route_prune_home_stop(p_tenant_id, v_old.route_id, v_rider, v_old.dropoff_stop_place_id, v_old_on);
        END IF;

        -- The days the row used to cover are re-planned too, on the route it used to be on.
        PERFORM tracking.fn_route_changed(v_old.route_id, v_old_on, v_old.valid_until);
    END IF;

    PERFORM tracking.fn_route_changed(v_route, v_on, v_until);
    RETURN v_id;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_assignment_put(uuid,uuid,jsonb) IS 'Creates (p_id NULL) or updates one assignment after every check the endpoints share, putting a new assignment''s empty leg stop, and any stop sent as "home", at the rider''s home (TRACK-037 D1, TRACK-044) or refusing rider.location-required (TRACK-044 D3); an update prunes the home stops it left (D5); writes route.changed for the days it covers and covered (TRACK-009 D4)';
