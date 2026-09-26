-- TRACK-025 step 1: what a live screen reads when it opens or resyncs, and who may listen to which
-- channel.
--
-- sp_live_fleet is every vehicle of the tenant that has reported, from vehicle_last_position (a
-- primary-key row per vehicle), with its trip in progress and that trip's route when it has one;
-- narrowed to an organization it keeps only the vehicles whose trip carries one of its riders.
--
-- sp_trip_live is the trip detail plus the vehicle's last position and the pending stops with their
-- coordinates — everything the ETA needs, in the same round-trip as the detail.
--
-- sp_can_subscribe answers one subscribe of the WebSocket gateway; the gateway caches the answer
-- five minutes per (user, channel). The access level is the gateway's to decide (a guardian passes
-- p_guardian); the SP decides what only the tenant's data can: the channel's tenant, trip, rider or
-- organization exists in it, and a guardian's rider is theirs and riding now.

CREATE FUNCTION tracking.sp_live_fleet(
    p_tenant_id UUID,
    p_organization_id UUID DEFAULT NULL
)
RETURNS TABLE(
    vehicle_id UUID,
    license_plate VARCHAR,
    trip_id UUID,
    route_name VARCHAR,
    lat FLOAT8,
    lng FLOAT8,
    speed REAL,
    heading REAL,
    recorded_at TIMESTAMPTZ
) AS $$
    SELECT
        v.id,
        CAST(v.plate_number AS VARCHAR),
        t.id,
        CAST(r.route_name AS VARCHAR),
        ST_Y(lp.location::geometry),
        ST_X(lp.location::geometry),
        lp.speed,
        lp.heading,
        lp.recorded_at
    FROM tracking.vehicle_last_position lp
    INNER JOIN tracking.vehicles v ON v.id = lp.vehicle_id
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    LEFT JOIN LATERAL (
        SELECT tr.id, tr.route_id
        FROM tracking.trips tr
        WHERE tr.vehicle_id = v.id
          AND tr.status = 'in_progress'
          AND tr.tenant_id = p_tenant_id
        ORDER BY tr.started_at DESC
        LIMIT 1
    ) t ON true
    LEFT JOIN tracking.routes r ON r.id = t.route_id
    WHERE p_organization_id IS NULL
       OR EXISTS (
            SELECT 1
            FROM tracking.trip_stops ts
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            INNER JOIN tracking.riders rd ON rd.id = k.subject_id
            WHERE ts.trip_id = t.id
              AND k.subject_type = 'passenger'
              AND k.status <> 'cancelled'
              AND rd.organization_id = p_organization_id
        )
    ORDER BY v.plate_number;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_live_fleet(UUID, UUID) IS
'Every reporting vehicle of the tenant with its last position, trip in progress and route; narrowed to the trips carrying riders of p_organization_id when set (TRACK-025)';

CREATE FUNCTION tracking.sp_trip_live(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    route_schedule_id UUID,
    service_date DATE,
    timezone VARCHAR,
    planned_start TIMESTAMPTZ,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_id UUID,
    driver_name VARCHAR,
    status VARCHAR,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    is_overridden BOOLEAN,
    stops_count INT,
    tasks_total INT,
    tasks_done INT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    stops JSONB,
    polyline TEXT,
    distance_km NUMERIC,
    trace_source VARCHAR,
    vehicle_position JSONB,
    pending_stops JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        g.*,
        CASE WHEN lp.vehicle_id IS NOT NULL THEN jsonb_build_object(
            'vehicle_id',  lp.vehicle_id,
            'trip_id',     lp.trip_id,
            'lat',         ST_Y(lp.location::geometry),
            'lng',         ST_X(lp.location::geometry),
            'speed',       lp.speed,
            'heading',     lp.heading,
            'accuracy',    lp.accuracy,
            'recorded_at', lp.recorded_at
        ) END,
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_stop_id', ts.id,
                'lat',          ST_Y(sp.location::geometry),
                'lng',          ST_X(sp.location::geometry)
            ) ORDER BY ts.sequence)
            FROM tracking.trip_stops ts
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = g.id
              AND ts.status = 'pending'
              AND sp.location IS NOT NULL
        ), '[]'::jsonb)
    FROM tracking.sp_get_trip_traced(p_tenant_id, p_trip_id) g
    LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = g.vehicle_id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_trip_live(UUID, UUID) IS
'The trip detail with its trace, plus its vehicle''s last position and its pending stops with coordinates, in sequence; raises trip.not-found (TRACK-025)';

-- Channels: fleet:{tenant} · fleet:{tenant}:org:{organization} · trip:{trip} · rider:{rider}.
CREATE FUNCTION tracking.sp_can_subscribe(
    p_tenant_id UUID,
    p_user_id UUID,
    p_guardian BOOLEAN,
    p_channel TEXT
)
RETURNS BOOLEAN AS $$
DECLARE
    v_parts TEXT[] := string_to_array(p_channel, ':');
    v_uuid  CONSTANT TEXT := '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    v_id    UUID;
BEGIN
    IF array_length(v_parts, 1) NOT IN (2, 4) OR v_parts[2] !~ v_uuid THEN
        RETURN FALSE;
    END IF;
    v_id := CAST(v_parts[2] AS UUID);

    IF v_parts[1] = 'fleet' THEN
        IF p_guardian OR v_id <> p_tenant_id THEN
            RETURN FALSE;
        END IF;
        IF array_length(v_parts, 1) = 2 THEN
            RETURN TRUE;
        END IF;
        RETURN v_parts[3] = 'org' AND v_parts[4] ~ v_uuid AND EXISTS (
            SELECT 1 FROM tracking.organizations o
            WHERE o.id = CAST(v_parts[4] AS UUID) AND o.tenant_id = p_tenant_id);
    END IF;

    IF array_length(v_parts, 1) <> 2 THEN
        RETURN FALSE;
    END IF;

    IF v_parts[1] = 'trip' THEN
        RETURN NOT p_guardian AND EXISTS (
            SELECT 1 FROM tracking.trips t WHERE t.id = v_id AND t.tenant_id = p_tenant_id);
    END IF;

    IF v_parts[1] = 'rider' THEN
        IF NOT p_guardian THEN
            RETURN EXISTS (
                SELECT 1 FROM tracking.riders rd
                INNER JOIN tracking.organizations o ON o.id = rd.organization_id
                WHERE rd.id = v_id AND o.tenant_id = p_tenant_id);
        END IF;
        RETURN v_id = ANY(tracking.fn_guardian_scope(p_tenant_id, p_user_id)) AND EXISTS (
            SELECT 1
            FROM tracking.trips t
            INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            WHERE t.tenant_id = p_tenant_id
              AND t.status = 'in_progress'
              AND k.subject_type = 'passenger'
              AND k.subject_id = v_id
              AND k.status <> 'cancelled');
    END IF;

    RETURN FALSE;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_can_subscribe(UUID, UUID, BOOLEAN, TEXT) IS
'Whether the user may subscribe to a realtime channel: fleet, org fleet and trip channels of the tenant for operators; a guardian only rider:{id} of their own rider while it rides a trip in progress (TRACK-025)';

-- Who a trip's changes go to besides its own channel and the fleet: its riders, and their
-- organizations' fleets (TRACK-025 step 4).
CREATE FUNCTION tracking.sp_trip_audience(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS TABLE(rider_ids UUID[], organization_ids UUID[]) AS $$
    SELECT
        COALESCE(array_agg(DISTINCT k.subject_id), '{}'::UUID[]),
        COALESCE(array_agg(DISTINCT rd.organization_id) FILTER (WHERE rd.organization_id IS NOT NULL), '{}'::UUID[])
    FROM tracking.trips t
    INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
    INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id AND k.subject_type = 'passenger'
    LEFT JOIN tracking.riders rd ON rd.id = k.subject_id
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_trip_audience(UUID, UUID) IS
'The riders of a trip and their organizations: the rider and organization-fleet channels its changes are published to (TRACK-025)';
