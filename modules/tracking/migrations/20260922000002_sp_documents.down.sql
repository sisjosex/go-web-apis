-- Reverse of TRACK-016 step 1's slice. The two fleet lists go back to what they returned before
-- service_blocked; everything else simply goes.
DROP FUNCTION IF EXISTS tracking.sp_create_document_type(UUID, VARCHAR, VARCHAR, VARCHAR, INT, BOOLEAN, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_update_document_type(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, INT, BOOLEAN, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_list_document_types(UUID, VARCHAR, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_create_document(UUID, VARCHAR, UUID, UUID, VARCHAR, DATE, DATE, TEXT);
DROP FUNCTION IF EXISTS tracking.sp_update_document(UUID, UUID, VARCHAR, DATE, DATE, TEXT);
DROP FUNCTION IF EXISTS tracking.sp_get_document(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_documents(UUID, VARCHAR, UUID, INT, INT, INT);
DROP FUNCTION IF EXISTS tracking.sp_delete_document(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_document_subject_name(UUID, VARCHAR, UUID);

DROP FUNCTION IF EXISTS tracking.sp_list_drivers(UUID, VARCHAR, UUID, VARCHAR, INT, INT);
CREATE FUNCTION tracking.sp_list_drivers(
    p_tenant_id  UUID,
    p_search     VARCHAR DEFAULT NULL,
    p_company_id UUID    DEFAULT NULL,
    p_status     VARCHAR DEFAULT NULL,
    p_page       INT     DEFAULT 1,
    p_page_size  INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    company_id UUID,
    company_name VARCHAR,
    user_id UUID,
    user_email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    license_number VARCHAR,
    license_class VARCHAR,
    license_expires_on DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        d.company_id,
        CAST(tc.name AS VARCHAR),
        d.user_id,
        CAST(u.email AS VARCHAR),
        CAST(d.first_name AS VARCHAR),
        CAST(d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(d.license_number AS VARCHAR),
        CAST(d.license_class AS VARCHAR),
        d.license_expires_on,
        CAST(d.status AS VARCHAR),
        d.created_at,
        d.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.drivers d
    INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
    LEFT JOIN auth.users u ON u.id = d.user_id AND u.deleted_at IS NULL
    WHERE d.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR d.company_id = p_company_id)
      AND (p_status IS NULL OR p_status = '' OR d.status = p_status)
      AND (p_search IS NULL OR p_search = ''
           OR d.first_name ILIKE '%' || p_search || '%'
           OR d.last_name ILIKE '%' || p_search || '%'
           OR d.license_number ILIKE '%' || p_search || '%')
    ORDER BY d.last_name ASC, d.first_name ASC, d.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_drivers(UUID, VARCHAR, UUID, VARCHAR, INT, INT) IS
'One page of a tenant''s drivers by surname, optionally narrowed by a name or licence search, carrier and status; every row carries its carrier, its linked account''s email and the total_count the filters match';

DROP FUNCTION IF EXISTS tracking.sp_list_vehicles(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, INT, INT);
CREATE FUNCTION tracking.sp_list_vehicles(
    p_tenant_id    UUID,
    p_company_id   UUID    DEFAULT NULL,
    p_vehicle_type VARCHAR DEFAULT NULL,
    p_status       VARCHAR DEFAULT NULL,
    p_search       VARCHAR DEFAULT NULL,
    p_page         INT     DEFAULT 1,
    p_page_size    INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    company_name VARCHAR,
    plate_number VARCHAR,
    vehicle_type VARCHAR,
    brand VARCHAR,
    model VARCHAR,
    year INT,
    capacity INT,
    gps_device_id VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        CAST(tc.name AS VARCHAR),
        CAST(v.plate_number AS VARCHAR),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(v.brand AS VARCHAR),
        CAST(v.model AS VARCHAR),
        v.year,
        v.capacity,
        CAST(v.gps_device_id AS VARCHAR),
        CAST(v.status AS VARCHAR),
        v.created_at,
        v.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR v.company_id = p_company_id)
      AND (p_vehicle_type IS NULL OR p_vehicle_type = '' OR v.vehicle_type = p_vehicle_type)
      AND (p_status IS NULL OR p_status = '' OR v.status = p_status)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR v.plate_number ILIKE '%' || p_search || '%'
      )
    ORDER BY v.plate_number ASC, v.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_vehicles(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, INT, INT) IS
'One page of a tenant''s vehicles by plate number, optionally narrowed by company, type, status and a plate search; every row carries its company name and the total_count the filters match. p_company_id NULL lists the whole fleet (TRACK-001 D2)';

DROP FUNCTION IF EXISTS tracking.fn_service_blocked(UUID, VARCHAR, UUID);
