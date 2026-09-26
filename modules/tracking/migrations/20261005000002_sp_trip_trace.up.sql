-- TRACK-010 step 4: a completed trip keeps its path as one line and its length, so reports and the
-- trip view never read raw points (which retention drops anyway).
--
-- The outbox:trip.changed handler, on `complete`, reads the trip's points with sp_trip_trace_points,
-- map-matches them (Valhalla trace_attributes) and stores the result with sp_trip_close_trace. The
-- router down or refusing: the raw GPS line is stored instead, trace_source 'fallback'. A trip with
-- fewer than two points keeps polyline NULL. Idempotent: a redelivered task writes the same line again.
--
-- GET /trips/:id answers them through sp_get_trip_traced, sp_get_trip plus the three columns: the
-- writes keep answering sp_get_trip's shape, so none of the transition SPs changes.

ALTER TABLE tracking.trips
    ADD COLUMN polyline TEXT,
    ADD COLUMN distance_km NUMERIC(8, 2),
    ADD COLUMN trace_source VARCHAR(20);

COMMENT ON COLUMN tracking.trips.polyline IS 'The driven path at trip close, polyline precision 6 (TRACK-010)';
COMMENT ON COLUMN tracking.trips.trace_source IS 'valhalla (map-matched) or fallback (the raw GPS line)';

-- The trip's points in order; bounded by its start and end so only those days' partitions are read.
CREATE FUNCTION tracking.sp_trip_trace_points(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS TABLE(lat FLOAT8, lng FLOAT8) AS $$
    SELECT ST_Y(p.location::geometry), ST_X(p.location::geometry)
    FROM tracking.trips t
    INNER JOIN tracking.vehicle_positions p
        ON p.trip_id = t.id
       AND p.recorded_at >= t.started_at
       AND p.recorded_at <= COALESCE(t.ended_at, now())
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id
      AND t.started_at IS NOT NULL
    ORDER BY p.recorded_at;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_trip_trace_points(UUID, UUID) IS
'The GPS points of a trip between its start and end, oldest first (TRACK-010)';

CREATE FUNCTION tracking.sp_trip_close_trace(
    p_tenant_id UUID,
    p_trip_id UUID,
    p_polyline TEXT,
    p_distance_km NUMERIC,
    p_source VARCHAR
)
RETURNS VOID AS $$
    UPDATE tracking.trips t
    SET polyline = p_polyline,
        distance_km = ROUND(p_distance_km, 2),
        trace_source = p_source,
        updated_at = now()
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id;
$$ LANGUAGE sql;

COMMENT ON FUNCTION tracking.sp_trip_close_trace(UUID, UUID, TEXT, NUMERIC, VARCHAR) IS
'Stores a trip''s driven path and its length in km, and where the line came from (TRACK-010)';

CREATE FUNCTION tracking.sp_get_trip_traced(
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
    trace_source VARCHAR
) AS $$
BEGIN
    RETURN QUERY
    SELECT g.*, t.polyline, CAST(t.distance_km AS NUMERIC), CAST(t.trace_source AS VARCHAR)
    FROM tracking.sp_get_trip(p_tenant_id, p_trip_id) g
    INNER JOIN tracking.trips t ON t.id = g.id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_get_trip_traced(UUID, UUID) IS
'sp_get_trip plus the trip''s driven path, its km and its source — GET /trips/:id; raises trip.not-found (TRACK-010)';
