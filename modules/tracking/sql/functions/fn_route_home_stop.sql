CREATE OR REPLACE FUNCTION tracking.fn_route_home_stop(p_tenant_id uuid, p_route_id uuid, p_rider_id uuid, p_on date)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
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

        v_target := tracking.fn_route_version_split(v_version.id, p_on);

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

    -- Every trip from p_on on calls at one more stop, leaves earlier (D2), and the line is re-traced.
    IF v_added THEN
        PERFORM tracking.fn_route_recompute_departure(p_tenant_id, p_route_id, p_on);
        PERFORM tracking.fn_route_changed(p_route_id, p_on, NULL);
    END IF;
    RETURN v_place;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_home_stop(uuid,uuid,uuid,date) IS 'Puts the rider''s home (a place within 30 m, or a new one) on every version of the route in force from p_on, at the cheapest straight-line position, splitting a version that started earlier, and moves the auto departure (TRACK-044 D2); NULL when the rider has no home pin (TRACK-037 D1); a rider with a shared stop (TRACK-043 D3) uses it instead';
