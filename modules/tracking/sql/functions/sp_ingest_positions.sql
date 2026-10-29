CREATE OR REPLACE FUNCTION tracking.sp_ingest_positions(p_tenant_id uuid, p_points jsonb, p_arrival_radius_m integer DEFAULT 50, p_approach_radius_m integer DEFAULT 800, p_phone_priority_s integer DEFAULT 90)
 RETURNS TABLE(vehicle_id uuid, trip_id uuid, last jsonb, rider_ids uuid[], organization_ids uuid[], arrived_stop_id uuid)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_arrival  RECORD;
    v_vehicles UUID[] := '{}';
    v_stops    UUID[] := '{}';
    v_approach RECORD;
    v_kept     JSONB;
    v_alert    tracking.route_alerts%ROWTYPE;
    v_settings RECORD;
    v_fix      RECORD;
    v_trip     UUID;
    v_count    INT;
    v_trips    UUID[] := '{}';
    v_counts   INT[] := '{}';
BEGIN
    -- The phone wins while it is alive (TRACK-068 D2): a device point is dropped when its vehicle is on a
    -- trip and the phone reported within p_phone_priority_s. Everything below reads only what is kept.
    WITH kept AS (
        SELECT p.value AS point
        FROM jsonb_array_elements(p_points) p
        WHERE COALESCE(p.value->>'source', '') <> 'device'
           OR NOT EXISTS (
                SELECT 1
                FROM tracking.vehicle_last_position lp
                WHERE lp.vehicle_id = CAST(p.value->>'vehicle_id' AS UUID)
                  AND lp.source = 'phone'
                  AND lp.recorded_at > now() - make_interval(secs => p_phone_priority_s))
           OR NOT EXISTS (
                SELECT 1
                FROM tracking.trips t
                WHERE t.vehicle_id = CAST(p.value->>'vehicle_id' AS UUID)
                  AND t.status = 'in_progress'
                  AND t.tenant_id = p_tenant_id)
    )
    SELECT COALESCE(jsonb_agg(kept.point), '[]'::JSONB) INTO v_kept FROM kept;

    -- A device point off any trip only moves the last position (D3): the trace exists for trips alone.
    INSERT INTO tracking.vehicle_positions AS vp (vehicle_id, recorded_at, location, speed, heading, accuracy, trip_id, source)
    SELECT i.vehicle_id, i.recorded_at, i.location, i.speed, i.heading, i.accuracy, i.trip_id, i.source
    FROM tracking.fn_ingest_points(p_tenant_id, v_kept) i
    WHERE i.source = 'phone' OR i.trip_id IS NOT NULL
    ON CONFLICT DO NOTHING;

    INSERT INTO tracking.vehicle_last_position AS lp (vehicle_id, recorded_at, location, speed, heading, accuracy, trip_id, source, updated_at)
    SELECT DISTINCT ON (i.vehicle_id) i.vehicle_id, i.recorded_at, i.location, i.speed, i.heading, i.accuracy, i.trip_id, i.source, now()
    FROM tracking.fn_ingest_points(p_tenant_id, v_kept) i
    WHERE EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = i.vehicle_id)
    ORDER BY i.vehicle_id, i.recorded_at DESC
    ON CONFLICT ON CONSTRAINT vehicle_last_position_pkey DO UPDATE
    SET recorded_at = EXCLUDED.recorded_at,
        location    = EXCLUDED.location,
        speed       = EXCLUDED.speed,
        heading     = EXCLUDED.heading,
        accuracy    = EXCLUDED.accuracy,
        trip_id     = EXCLUDED.trip_id,
        source      = EXCLUDED.source,
        updated_at  = now()
    WHERE EXCLUDED.recorded_at > lp.recorded_at;

    -- A position back resolves the vehicle's silent alert (TRACK-071 D2).
    FOR v_alert IN
        UPDATE tracking.route_alerts a
        SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
        WHERE a.tenant_id = p_tenant_id
          AND a.status = 'active'
          AND a.alert_type = 'device_silent'
          AND a.vehicle_id IN (SELECT i.vehicle_id FROM tracking.fn_ingest_points(p_tenant_id, v_kept) i)
        RETURNING a.*
    LOOP
        PERFORM tracking.fn_alert_changed(v_alert, 'resolved');
    END LOOP;

    -- Off the planned line (TRACK-071 D1): each trip's fixes in order against its version's line; the
    -- trip's counter runs positive off the line and negative on it, so off_route_fixes in a row raise the
    -- alert and as many back on resolve it. A fallback (straight) line has no planned_line: never.
    SELECT * INTO v_settings FROM tracking.sp_alert_settings_get(p_tenant_id);
    FOR v_fix IN
        SELECT i.trip_id, t.route_id, t.vehicle_id, t.off_route_fixes, i.location,
               NOT ST_DWithin(i.location, rv.planned_line, v_settings.off_route_m) AS off
        FROM tracking.fn_ingest_points(p_tenant_id, v_kept) i
        INNER JOIN tracking.trips t ON t.id = i.trip_id
        INNER JOIN tracking.route_versions rv ON rv.id = t.route_version_id
        WHERE rv.planned_line IS NOT NULL
          AND i.recorded_at >= t.started_at
        ORDER BY i.trip_id, i.recorded_at
    LOOP
        IF v_trip IS DISTINCT FROM v_fix.trip_id THEN
            IF v_trip IS NOT NULL THEN
                v_trips := v_trips || v_trip;
                v_counts := v_counts || v_count;
            END IF;
            v_trip := v_fix.trip_id;
            v_count := v_fix.off_route_fixes;
        END IF;
        v_count := CASE WHEN v_fix.off THEN GREATEST(v_count, 0) + 1 ELSE LEAST(v_count, 0) - 1 END;
        v_count := LEAST(GREATEST(v_count, -100), 100);

        IF v_count >= v_settings.off_route_fixes AND NOT EXISTS (
            SELECT 1 FROM tracking.route_alerts a
            WHERE a.trip_id = v_fix.trip_id AND a.alert_type = 'off_route' AND a.status = 'active'
        ) THEN
            INSERT INTO tracking.route_alerts (tenant_id, route_id, vehicle_id, trip_id, alert_type, title, message, severity, location, status)
            VALUES (p_tenant_id, v_fix.route_id, v_fix.vehicle_id, v_fix.trip_id, 'off_route', 'off_route', '', 'medium', v_fix.location, 'active')
            RETURNING * INTO v_alert;
            PERFORM tracking.fn_alert_changed(v_alert, 'raised');
        ELSIF v_count <= -v_settings.off_route_fixes THEN
            FOR v_alert IN
                UPDATE tracking.route_alerts a
                SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
                WHERE a.trip_id = v_fix.trip_id AND a.alert_type = 'off_route' AND a.status = 'active'
                RETURNING a.*
            LOOP
                PERFORM tracking.fn_alert_changed(v_alert, 'resolved');
            END LOOP;
        END IF;
    END LOOP;
    IF v_trip IS NOT NULL THEN
        v_trips := v_trips || v_trip;
        v_counts := v_counts || v_count;
        UPDATE tracking.trips t
        SET off_route_fixes = c.n
        FROM unnest(v_trips, v_counts) AS c(trip_id, n)
        WHERE t.id = c.trip_id;
    END IF;

    FOR v_approach IN
        UPDATE tracking.trip_stops ts
        SET approach_notified_at = now()
        FROM (
            SELECT DISTINCT ON (i.vehicle_id) i.trip_id, ns.id AS stop_id
            FROM tracking.fn_ingest_points(p_tenant_id, v_kept) i
            INNER JOIN tracking.trips t ON t.id = i.trip_id
            CROSS JOIN LATERAL (
                SELECT s.id, s.approach_notified_at, sp.location
                FROM tracking.trip_stops s
                INNER JOIN tracking.stop_places sp ON sp.id = s.stop_place_id
                WHERE s.trip_id = t.id AND s.status = 'pending'
                ORDER BY s.sequence
                LIMIT 1
            ) ns
            WHERE i.recorded_at >= t.started_at
              AND ns.approach_notified_at IS NULL
              AND ns.location IS NOT NULL
              AND ST_DWithin(i.location, ns.location, p_approach_radius_m)
            ORDER BY i.vehicle_id, i.recorded_at
        ) a
        WHERE ts.id = a.stop_id
          AND ts.approach_notified_at IS NULL
        RETURNING ts.trip_id, ts.id
    LOOP
        PERFORM tracking.fn_trip_changed(p_tenant_id, v_approach.trip_id, 'approach', NULL,
            jsonb_build_object('trip_stop_id', v_approach.id));
    END LOOP;

    FOR v_arrival IN
        SELECT DISTINCT ON (i.vehicle_id) i.vehicle_id AS vid, ns.id AS stop_id
        FROM tracking.fn_ingest_points(p_tenant_id, v_kept) i
        INNER JOIN tracking.trips t ON t.id = i.trip_id
        CROSS JOIN LATERAL (
            SELECT ts.id, sp.location
            FROM tracking.trip_stops ts
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = t.id AND ts.status = 'pending'
            ORDER BY ts.sequence
            LIMIT 1
        ) ns
        WHERE i.recorded_at >= t.started_at
          AND ns.location IS NOT NULL
          AND ST_DWithin(i.location, ns.location, p_arrival_radius_m)
        ORDER BY i.vehicle_id
    LOOP
        BEGIN
            PERFORM 1 FROM tracking.sp_trip_stop_transition(p_tenant_id, v_arrival.stop_id, 'arrive', NULL);
            v_vehicles := v_vehicles || v_arrival.vid;
            v_stops := v_stops || v_arrival.stop_id;
        EXCEPTION WHEN raise_exception THEN
            NULL;
        END;
    END LOOP;

    RETURN QUERY
    SELECT
        lp.vehicle_id,
        tr.trip_id,
        jsonb_build_object(
            'vehicle_id',  lp.vehicle_id,
            'trip_id',     lp.trip_id,
            'lat',         ST_Y(lp.location::geometry),
            'lng',         ST_X(lp.location::geometry),
            'speed',       lp.speed,
            'heading',     lp.heading,
            'accuracy',    lp.accuracy,
            'source',      lp.source,
            'recorded_at', lp.recorded_at
        ),
        COALESCE(rs.riders, '{}'::UUID[]),
        COALESCE(rs.organizations, '{}'::UUID[]),
        v_stops[array_position(v_vehicles, lp.vehicle_id)]
    FROM (
        SELECT DISTINCT ON (i.vehicle_id) i.vehicle_id, i.trip_id
        FROM tracking.fn_ingest_points(p_tenant_id, v_kept) i
        ORDER BY i.vehicle_id
    ) tr
    INNER JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = tr.vehicle_id
    -- The riders still aboard or waiting (D1), and every rider's organization.
    LEFT JOIN LATERAL (
        SELECT array_agg(DISTINCT k.subject_id) FILTER (WHERE NOT EXISTS (
                   SELECT 1
                   FROM tracking.trip_stop_tasks f
                   INNER JOIN tracking.trip_stops fs ON fs.id = f.trip_stop_id
                   WHERE fs.trip_id = tr.trip_id
                     AND f.subject_type = 'passenger'
                     AND f.subject_id = k.subject_id
                     AND ((f.kind = 'dropoff' AND f.status IN ('done', 'no_show'))
                       OR (f.kind = 'pickup' AND f.status = 'no_show')))) AS riders,
               array_agg(DISTINCT rd.organization_id) FILTER (WHERE rd.organization_id IS NOT NULL) AS organizations
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        LEFT JOIN tracking.riders rd ON rd.id = k.subject_id
        WHERE ts.trip_id = tr.trip_id
          AND k.subject_type = 'passenger'
          AND k.status <> 'cancelled'
    ) rs ON true;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_ingest_positions(uuid,jsonb,integer,integer,integer) IS 'Stores a batch of GPS points (idempotent on vehicle_id + recorded_at), moves each vehicle''s last position forward, tags points with the trip in progress and marks the next stop arrived within p_arrival_radius_m, and writes one trip.changed approach the first time a vehicle comes within p_approach_radius_m of it; answers per vehicle { trip_id, last, rider_ids (riders not yet off, TRACK-030), organization_ids, arrived_stop_id }; a device point yields to a phone that reported within p_phone_priority_s during a trip, and off a trip only moves the last position (TRACK-010, TRACK-012, TRACK-068); a position resolves the vehicle''s device_silent alert, and off_route_fixes fixes in a row farther than off_route_m from the version''s planned_line raise off_route, as many back on resolve it (TRACK-071)';
