DROP FUNCTION IF EXISTS tracking.sp_get_route_path(UUID, UUID, DATE, UUID);
DROP FUNCTION IF EXISTS tracking.sp_set_route_version_path(UUID, UUID, TEXT, INT, JSONB, VARCHAR, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_route_version_path_points(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_route_paths_backfill(UUID);
DROP FUNCTION IF EXISTS tracking.sp_route_path_versions(UUID, UUID, DATE);

DROP FUNCTION tracking.sp_live_fleet(UUID, UUID);

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

ALTER TABLE tracking.route_versions
    DROP CONSTRAINT IF EXISTS chk_route_versions_path_source,
    DROP COLUMN IF EXISTS path_computed_at,
    DROP COLUMN IF EXISTS path_points_hash,
    DROP COLUMN IF EXISTS path_source,
    DROP COLUMN IF EXISTS planned_legs,
    DROP COLUMN IF EXISTS planned_distance_m,
    DROP COLUMN IF EXISTS planned_polyline;
