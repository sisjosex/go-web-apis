-- Rider assignment operations with tenant_id validation

CREATE OR REPLACE FUNCTION tracking.sp_assign_rider(
    p_tenant_id UUID,
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
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_rider_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.rider_assignments (
        rider_id, route_id, pickup_stop_id, dropoff_stop_id, status
    )
    VALUES (
        p_rider_id,
        p_route_id,
        p_pickup_stop_id,
        p_dropoff_stop_id,
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING
        tracking.rider_assignments.id,
        tracking.rider_assignments.rider_id,
        tracking.rider_assignments.route_id,
        tracking.rider_assignments.pickup_stop_id,
        tracking.rider_assignments.dropoff_stop_id,
        CAST(tracking.rider_assignments.status AS VARCHAR),
        tracking.rider_assignments.created_at,
        tracking.rider_assignments.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_rider_assignments(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_route_id UUID,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    route_id UUID,
    pickup_stop_id UUID,
    dropoff_stop_id UUID,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        ra.id,
        ra.rider_id,
        ra.route_id,
        ra.pickup_stop_id,
        ra.dropoff_stop_id,
        CAST(ra.status AS VARCHAR),
        ra.created_at,
        ra.updated_at
    FROM tracking.rider_assignments ra
    INNER JOIN tracking.riders r ON r.id = ra.rider_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_rider_id IS NULL OR ra.rider_id = p_rider_id)
      AND (p_route_id IS NULL OR ra.route_id = p_route_id)
      AND (p_is_active IS NULL OR (ra.status = 'active') = p_is_active)
    ORDER BY ra.created_at DESC;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_unassign_rider(
    p_tenant_id UUID,
    p_assignment_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.rider_assignments ra
        INNER JOIN tracking.riders r ON r.id = ra.rider_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE ra.id = p_assignment_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.rider_assignments ra WHERE ra.id = p_assignment_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;
