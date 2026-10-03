-- TRACK-034 rollback: inline places, conflicts, the vehicle filter and the derived duration go.

DROP FUNCTION tracking.sp_vehicle_schedule_conflicts(UUID, UUID, UUID, UUID);
DROP FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID, UUID);

CREATE FUNCTION tracking.sp_list_routes(
    p_tenant_id     UUID,
    p_search        VARCHAR DEFAULT NULL,
    p_company_id    UUID    DEFAULT NULL,
    p_direction     VARCHAR DEFAULT NULL,
    p_is_active     BOOLEAN DEFAULT NULL,
    p_page          INT     DEFAULT 1,
    p_page_size     INT     DEFAULT 20,
    p_scope_user_id UUID    DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    company_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    default_driver_id UUID,
    driver_name VARCHAR,
    route_name VARCHAR,
    route_code VARCHAR,
    direction VARCHAR,
    origin_address VARCHAR,
    origin_lat DECIMAL,
    origin_lng DECIMAL,
    destination_address VARCHAR,
    destination_lat DECIMAL,
    destination_lng DECIMAL,
    estimated_duration_minutes INT,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    capacity INT,
    capacity_override INT,
    total_count BIGINT
) LANGUAGE plpgsql AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        CAST(tc.name AS VARCHAR),
        r.vehicle_id,
        CAST(v.plate_number AS VARCHAR),
        r.default_driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.direction AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        r.estimated_duration_minutes,
        CAST(r.timezone AS VARCHAR),
        r.is_active,
        r.created_at,
        r.updated_at,
        COALESCE(r.capacity, v.capacity),
        r.capacity,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.routes r
    -- Not a LEFT JOIN: this join is what scopes the rows to the tenant.
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN tracking.drivers d ON d.id = r.default_driver_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_direction IS NULL OR p_direction = '' OR r.direction = p_direction)
      AND (p_is_active IS NULL OR r.is_active = p_is_active)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR r.route_name ILIKE '%' || p_search || '%'
          OR r.route_code ILIKE '%' || p_search || '%'
      )
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.rider_route_assignments ra
          INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
          WHERE ra.route_id = r.id
            AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
            AND rr.organization_id = ANY(v_scope)
      ))
    -- Alphabetical, cut server-side; r.id breaks ties so two routes of one name never swap pages.
    ORDER BY r.route_name ASC, r.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID) IS
'One page of the tenant''s routes by name, narrowed by a name/code search, company, direction and active flag; every row carries company name, plate, driver name and the total_count the filters match (TRACK-002 D2)';

CREATE OR REPLACE FUNCTION tracking.fn_route_version_stops_json(
    p_tenant_id UUID,
    p_version_id UUID,
    p_stops JSONB
)
RETURNS INT AS $$
DECLARE
    v_inserted INT;
BEGIN
    IF EXISTS (
        SELECT 1
        FROM jsonb_array_elements(COALESCE(p_stops, '[]'::jsonb)) e
        WHERE NOT EXISTS (
            SELECT 1 FROM tracking.stop_places sp
            WHERE sp.id = (e->>'stop_place_id')::UUID
              AND sp.tenant_id = p_tenant_id
        )
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_version_stops (version_id, stop_place_id, sequence, planned_offset_min, dwell_sec)
    SELECT
        p_version_id,
        (e.value->>'stop_place_id')::UUID,
        CAST(ROW_NUMBER() OVER (ORDER BY COALESCE((e.value->>'sequence')::INT, CAST(e.ordinality AS INT)), e.ordinality) AS INT),
        (e.value->>'planned_offset_min')::INT,
        (e.value->>'dwell_sec')::INT
    FROM jsonb_array_elements(COALESCE(p_stops, '[]'::jsonb)) WITH ORDINALITY AS e(value, ordinality);
    GET DIAGNOSTICS v_inserted = ROW_COUNT;
    RETURN v_inserted;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_set_route_version_path(
    p_tenant_id UUID,
    p_version_id UUID,
    p_polyline TEXT,
    p_distance_m INT,
    p_legs JSONB,
    p_source VARCHAR,
    p_points_hash VARCHAR
)
RETURNS VOID AS $$
    UPDATE tracking.route_versions rv
    SET planned_polyline   = p_polyline,
        planned_distance_m = p_distance_m,
        planned_legs       = p_legs,
        path_source        = p_source,
        path_points_hash   = p_points_hash,
        path_computed_at   = clock_timestamp()
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE rv.id = p_version_id
      AND r.id = rv.route_id
      AND tc.tenant_id = p_tenant_id;
$$ LANGUAGE sql;
