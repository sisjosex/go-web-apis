-- Vehicle operations with tenant_id validation via transport_companies JOIN

CREATE OR REPLACE FUNCTION tracking.sp_create_vehicle(
    p_tenant_id UUID,
    p_company_id UUID,
    p_plate_number VARCHAR(50),
    p_vehicle_type VARCHAR(50),
    p_brand VARCHAR(100),
    p_model VARCHAR(100),
    p_year INT,
    p_capacity INT,
    p_gps_device_id VARCHAR(100),
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
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.vehicles v
        WHERE v.company_id = p_company_id
          AND v.plate_number = TRIM(p_plate_number)
    ) THEN
        RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.vehicles (
        company_id, plate_number, vehicle_type, brand, model, year, capacity, gps_device_id, status
    )
    VALUES (
        p_company_id,
        TRIM(p_plate_number),
        p_vehicle_type,
        NULLIF(TRIM(p_brand), ''),
        NULLIF(TRIM(p_model), ''),
        p_year,
        COALESCE(p_capacity, 0),
        NULLIF(TRIM(p_gps_device_id), ''),
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING
        tracking.vehicles.id,
        tracking.vehicles.company_id,
        CAST(tracking.vehicles.plate_number AS VARCHAR),
        CAST(tracking.vehicles.vehicle_type AS VARCHAR),
        CAST(tracking.vehicles.brand AS VARCHAR),
        CAST(tracking.vehicles.model AS VARCHAR),
        tracking.vehicles.year,
        tracking.vehicles.capacity,
        CAST(tracking.vehicles.gps_device_id AS VARCHAR),
        CAST(tracking.vehicles.status AS VARCHAR),
        tracking.vehicles.created_at,
        tracking.vehicles.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_update_vehicle(
    p_tenant_id UUID,
    p_vehicle_id UUID,
    p_plate_number VARCHAR(50),
    p_vehicle_type VARCHAR(50),
    p_brand VARCHAR(100),
    p_model VARCHAR(100),
    p_year INT,
    p_capacity INT,
    p_gps_device_id VARCHAR(100),
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
    IF NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.vehicles v SET
        plate_number = COALESCE(NULLIF(TRIM(p_plate_number), ''), v.plate_number),
        vehicle_type = COALESCE(NULLIF(TRIM(p_vehicle_type), ''), v.vehicle_type),
        brand = NULLIF(TRIM(p_brand), ''),
        model = NULLIF(TRIM(p_model), ''),
        year = COALESCE(p_year, v.year),
        capacity = COALESCE(p_capacity, v.capacity),
        gps_device_id = NULLIF(TRIM(p_gps_device_id), ''),
        status = COALESCE(NULLIF(TRIM(p_status), ''), v.status),
        updated_at = CURRENT_TIMESTAMP
    WHERE v.id = p_vehicle_id
    RETURNING
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
        v.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_vehicles(
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

CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle(
    p_tenant_id UUID,
    p_vehicle_id UUID
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
    IF NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

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
    WHERE v.id = p_vehicle_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_delete_vehicle(
    p_tenant_id UUID,
    p_vehicle_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.vehicles v WHERE v.id = p_vehicle_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;
