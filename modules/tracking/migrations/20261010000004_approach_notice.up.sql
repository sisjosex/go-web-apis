-- TRACK-012 step 5 (D4): "the bus is near" is a distance, checked where every position already lands.
--
-- trip_stops.approach_notified_at is when the stop's approach was announced; sp_ingest_positions
-- stamps it the first time a vehicle comes within TRACKING_APPROACH_RADIUS_M (800) of the trip's next
-- pending stop and writes a trip.changed `approach` through fn_trip_changed, which sp_notify_event
-- turns into an `approaching` notice to the riders with a pending task there. The column is the
-- once-per-trip guarantee: a stop is announced once, however many points fall inside.
--
-- The SP gains p_approach_radius_m, so DROP + CREATE; the body is the TRACK-010 one plus the
-- approach loop.

ALTER TABLE tracking.trip_stops ADD COLUMN approach_notified_at TIMESTAMPTZ;

COMMENT ON COLUMN tracking.trip_stops.approach_notified_at IS
'When the stop''s approach notice went out: the first position within the approach radius while it was the trip''s next pending stop (TRACK-012 D4)';

DROP FUNCTION IF EXISTS tracking.sp_ingest_positions(UUID, JSONB, INT);

CREATE FUNCTION tracking.sp_ingest_positions(
    p_tenant_id UUID,
    p_points JSONB,
    p_arrival_radius_m INT DEFAULT 50,
    p_approach_radius_m INT DEFAULT 800
)
RETURNS TABLE(vehicle_id UUID, trip_id UUID, last JSONB, rider_ids UUID[], organization_ids UUID[], arrived_stop_id UUID) AS $$
DECLARE
    v_arrival  RECORD;
    v_vehicles UUID[] := '{}';
    v_stops    UUID[] := '{}';
    v_approach RECORD;
BEGIN
    INSERT INTO tracking.vehicle_positions AS vp (vehicle_id, recorded_at, location, speed, heading, accuracy, trip_id)
    SELECT i.vehicle_id, i.recorded_at, i.location, i.speed, i.heading, i.accuracy, i.trip_id
    FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
    ON CONFLICT DO NOTHING;

    INSERT INTO tracking.vehicle_last_position AS lp (vehicle_id, recorded_at, location, speed, heading, accuracy, trip_id, updated_at)
    SELECT DISTINCT ON (i.vehicle_id) i.vehicle_id, i.recorded_at, i.location, i.speed, i.heading, i.accuracy, i.trip_id, now()
    FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
    WHERE EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = i.vehicle_id)
    ORDER BY i.vehicle_id, i.recorded_at DESC
    ON CONFLICT ON CONSTRAINT vehicle_last_position_pkey DO UPDATE
    SET recorded_at = EXCLUDED.recorded_at,
        location    = EXCLUDED.location,
        speed       = EXCLUDED.speed,
        heading     = EXCLUDED.heading,
        accuracy    = EXCLUDED.accuracy,
        trip_id     = EXCLUDED.trip_id,
        updated_at  = now()
    WHERE EXCLUDED.recorded_at > lp.recorded_at;

    -- The approach notice (TRACK-012 D4): the first point of a batch within p_approach_radius_m of the
    -- trip's next pending stop stamps it, once — approach_notified_at IS NULL is the guard, so two
    -- points inside, in one batch or two, write one trip.changed `approach`. Checked before the
    -- arrival, on the same next stop, so a bus that jumps straight inside the arrival radius still
    -- announces it. One comparison per vehicle, no routing call.
    FOR v_approach IN
        UPDATE tracking.trip_stops ts
        SET approach_notified_at = now()
        FROM (
            SELECT DISTINCT ON (i.vehicle_id) i.trip_id, ns.id AS stop_id
            FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
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
        FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
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
            'recorded_at', lp.recorded_at
        ),
        COALESCE(rs.riders, '{}'::UUID[]),
        COALESCE(rs.organizations, '{}'::UUID[]),
        v_stops[array_position(v_vehicles, lp.vehicle_id)]
    FROM (
        SELECT DISTINCT ON (i.vehicle_id) i.vehicle_id, i.trip_id
        FROM tracking.fn_ingest_points(p_tenant_id, p_points) i
        ORDER BY i.vehicle_id
    ) tr
    INNER JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = tr.vehicle_id
    -- Who is on the trip, and whose organizations they belong to: the rider and org channels.
    LEFT JOIN LATERAL (
        SELECT array_agg(DISTINCT k.subject_id) AS riders,
               array_agg(DISTINCT rd.organization_id) FILTER (WHERE rd.organization_id IS NOT NULL) AS organizations
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        LEFT JOIN tracking.riders rd ON rd.id = k.subject_id
        WHERE ts.trip_id = tr.trip_id
          AND k.subject_type = 'passenger'
          AND k.status <> 'cancelled'
    ) rs ON true;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_ingest_positions(UUID, JSONB, INT, INT) IS
'Stores a batch of GPS points (idempotent on vehicle_id + recorded_at), moves each vehicle''s last position forward, tags points with the trip in progress and marks the next stop arrived within p_arrival_radius_m, and writes one trip.changed approach the first time a vehicle comes within p_approach_radius_m of it; answers per vehicle { trip_id, last, rider_ids, organization_ids, arrived_stop_id } (TRACK-010, TRACK-012)';
