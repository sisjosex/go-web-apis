-- ASSIGN RIDER
CREATE OR REPLACE FUNCTION tracking.sp_assign_rider(
    p_rider_id UUID,
    p_route_id UUID,
    p_pickup_stop_id UUID,
    p_dropoff_stop_id UUID,
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    route_id UUID,
    pickup_stop_id UUID,
    dropoff_stop_id UUID,
    status VARCHAR(50),
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    -- Check if rider exists
    IF NOT EXISTS (SELECT 1 FROM tracking.riders tr WHERE tr.id = p_rider_id) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes tr WHERE tr.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if stops exist
    IF p_pickup_stop_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tracking.route_stops rs WHERE rs.id = p_pickup_stop_id) THEN
        RAISE EXCEPTION 'route.stop-not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_dropoff_stop_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tracking.route_stops rs WHERE rs.id = p_dropoff_stop_id) THEN
        RAISE EXCEPTION 'route.stop-not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.rider_assignments (
        rider_id, route_id, pickup_stop_id, dropoff_stop_id, status
    )
    VALUES (
        p_rider_id, p_route_id, p_pickup_stop_id, p_dropoff_stop_id, p_status
    )
    RETURNING
        tracking.rider_assignments.id,
        tracking.rider_assignments.rider_id,
        tracking.rider_assignments.route_id,
        tracking.rider_assignments.pickup_stop_id,
        tracking.rider_assignments.dropoff_stop_id,
        tracking.rider_assignments.status,
        tracking.rider_assignments.created_at,
        tracking.rider_assignments.updated_at;
END;
$$ LANGUAGE plpgsql;
