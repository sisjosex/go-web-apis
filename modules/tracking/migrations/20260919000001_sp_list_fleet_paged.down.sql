-- Restores the two fleet list SPs as they stood before TRACK-001: unpaged, no total_count,
-- companies filtered by registration number only, vehicles without company_name.

DROP INDEX IF EXISTS tracking.idx_vehicles_plate_trgm;
DROP INDEX IF EXISTS tracking.idx_vehicles_company_plate;
DROP INDEX IF EXISTS tracking.idx_transport_companies_reg_trgm;
DROP INDEX IF EXISTS tracking.idx_transport_companies_name_trgm;
DROP INDEX IF EXISTS tracking.idx_transport_companies_tenant_name;

DROP FUNCTION IF EXISTS tracking.sp_list_companies(UUID, VARCHAR, VARCHAR, INT, INT);
CREATE FUNCTION tracking.sp_list_companies(
    p_tenant_id UUID,
    p_registration_number VARCHAR(255),
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    name VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    address TEXT,
    city VARCHAR,
    country VARCHAR,
    registration_number VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        tc.id,
        tc.tenant_id,
        CAST(tc.name AS VARCHAR),
        CAST(tc.email AS VARCHAR),
        CAST(tc.phone AS VARCHAR),
        tc.address,
        CAST(tc.city AS VARCHAR),
        CAST(tc.country AS VARCHAR),
        CAST(tc.registration_number AS VARCHAR),
        CAST(tc.status AS VARCHAR),
        tc.created_at,
        tc.updated_at
    FROM tracking.transport_companies tc
    WHERE tc.tenant_id = p_tenant_id
      AND (p_registration_number IS NULL OR tc.registration_number ILIKE '%' || p_registration_number || '%')
      AND (p_status IS NULL OR tc.status = p_status)
    ORDER BY tc.created_at DESC;
END;
$$ LANGUAGE plpgsql;

DROP FUNCTION IF EXISTS tracking.sp_list_vehicles(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, INT, INT);
CREATE FUNCTION tracking.sp_list_vehicles(
    p_tenant_id UUID,
    p_company_id UUID,
    p_vehicle_type VARCHAR(50),
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    plate_number VARCHAR,
    vehicle_type VARCHAR,
    brand VARCHAR,
    model VARCHAR,
    year INT,
    capacity INT,
    gps_device_id VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        CAST(v.plate_number AS VARCHAR),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(v.brand AS VARCHAR),
        CAST(v.model AS VARCHAR),
        v.year,
        v.capacity,
        CAST(v.gps_device_id AS VARCHAR),
        CAST(v.status AS VARCHAR),
        v.created_at,
        v.updated_at
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR v.company_id = p_company_id)
      AND (p_vehicle_type IS NULL OR v.vehicle_type = p_vehicle_type)
      AND (p_status IS NULL OR v.status = p_status)
    ORDER BY v.created_at DESC;
END;
$$ LANGUAGE plpgsql;
