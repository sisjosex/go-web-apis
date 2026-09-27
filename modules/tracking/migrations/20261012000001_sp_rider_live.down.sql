-- Back to TRACK-025's sp_trip_live: stops without coordinates.
DROP FUNCTION IF EXISTS tracking.sp_rider_live(UUID, UUID, UUID, UUID);

CREATE OR REPLACE FUNCTION tracking.sp_trip_live(
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
