-- Migration: sp_company_read_operations
-- Module: tracking
-- Created: 2025-12-04 21:57:43

-- LIST COMPANIES
CREATE OR REPLACE FUNCTION tracking.sp_list_companies(
    p_registration_number VARCHAR(100),
    p_status VARCHAR(50)
)
RETURNS TABLE(
    out_id UUID,
    out_name VARCHAR(255),
    out_email VARCHAR(255),
    out_phone VARCHAR(20),
    out_address TEXT,
    out_city VARCHAR(100),
    out_country VARCHAR(100),
    out_registration_number VARCHAR(100),
    out_status VARCHAR(50),
    out_created_at TIMESTAMP,
    out_updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        id,
        name,
        email,
        phone,
        address,
        city,
        country,
        registration_number,
        status,
        created_at,
        updated_at
    FROM tracking.transport_companies
    WHERE 
        (p_registration_number IS NULL OR registration_number = p_registration_number)
        AND (p_status IS NULL OR status = p_status)
    ORDER BY created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- GET COMPANY BY ID
CREATE OR REPLACE FUNCTION tracking.sp_get_company(
    p_company_id UUID
)
RETURNS TABLE(
    out_id UUID,
    out_name VARCHAR(255),
    out_email VARCHAR(255),
    out_phone VARCHAR(20),
    out_address TEXT,
    out_city VARCHAR(100),
    out_country VARCHAR(100),
    out_registration_number VARCHAR(100),
    out_status VARCHAR(50),
    out_created_at TIMESTAMP,
    out_updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        id,
        name,
        email,
        phone,
        address,
        city,
        country,
        registration_number,
        status,
        created_at,
        updated_at
    FROM tracking.transport_companies
    WHERE id = p_company_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql;

-- DELETE COMPANY
CREATE OR REPLACE FUNCTION tracking.sp_delete_company(
    p_company_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_deleted BOOLEAN;
BEGIN
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE id = p_company_id) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if company has associated vehicles
    IF EXISTS (SELECT 1 FROM tracking.vehicles WHERE company_id = p_company_id) THEN
        RAISE EXCEPTION 'company.has-vehicles' USING ERRCODE = 'P0001';
    END IF;

    -- Delete the company
    DELETE FROM tracking.transport_companies WHERE id = p_company_id;
    
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted > 0;
END;
$$ LANGUAGE plpgsql;
