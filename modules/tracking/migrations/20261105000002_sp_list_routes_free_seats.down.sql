-- TRACK-046 down: sp_list_routes as TRACK-038 left it.

DROP FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID, UUID, UUID, BOOLEAN, VARCHAR);

CREATE FUNCTION tracking.sp_list_routes(
    p_tenant_id     UUID,
    p_search        VARCHAR DEFAULT NULL,
    p_company_id    UUID    DEFAULT NULL,
    p_direction     VARCHAR DEFAULT NULL,
    p_is_active     BOOLEAN DEFAULT NULL,
    p_page          INT     DEFAULT 1,
    p_page_size     INT     DEFAULT 20,
    p_scope_user_id UUID    DEFAULT NULL,
    p_vehicle_id    UUID    DEFAULT NULL,
    p_organization_id UUID  DEFAULT NULL
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
    organization_id UUID,
    organization_name VARCHAR,
    paired_route_id UUID,
    seats INT,
    assigned_count INT,
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
        r.organization_id,
        CAST(o.name AS VARCHAR),
        r.paired_route_id,
        v.capacity,
        -- Riders the route carries today: the "32/40 seats" of its summary (TRACK-038 D3).
        CAST((
            SELECT COUNT(DISTINCT ra.rider_id) FROM tracking.rider_route_assignments ra
            WHERE ra.route_id = r.id
              AND ra.valid_from <= CURRENT_DATE
              AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
        ) AS INT),
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.routes r
    -- Not a LEFT JOIN: this join is what scopes the rows to the tenant.
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.drivers d ON d.id = r.default_driver_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_vehicle_id IS NULL OR r.vehicle_id = p_vehicle_id)
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
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

COMMENT ON FUNCTION tracking.sp_list_routes(UUID, VARCHAR, UUID, VARCHAR, BOOLEAN, INT, INT, UUID, UUID, UUID) IS
'One page of the tenant''s routes by name, narrowed by a name/code search, company, vehicle (TRACK-034), direction and active flag; every row carries company name, plate, driver name and the total_count the filters match (TRACK-002 D2)';
