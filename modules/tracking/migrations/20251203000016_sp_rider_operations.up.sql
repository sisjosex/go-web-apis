-- CREATE RIDER
CREATE OR REPLACE FUNCTION tracking.sp_create_rider(
    p_company_id UUID,
    p_rider_type VARCHAR(50),
    p_first_name VARCHAR(100),
    p_last_name VARCHAR(100),
    p_guardian_user_id UUID,
    p_phone VARCHAR(50),
    p_email VARCHAR(255),
    p_address VARCHAR(500)
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    rider_type VARCHAR(50),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    guardian_user_id UUID,
    phone VARCHAR(50),
    email VARCHAR(255),
    address VARCHAR(500),
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE tracking.transport_companies.id = p_company_id) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.riders (
        company_id, first_name, last_name, email, phone, identification_number, emergency_contact, status
    )
    VALUES (
        p_company_id, p_first_name, p_last_name, p_email, p_phone, NULL, NULL, 'active'
    )
    RETURNING
        tracking.riders.id,
        tracking.riders.company_id,
        p_rider_type,
        tracking.riders.first_name,
        tracking.riders.last_name,
        p_guardian_user_id,
        tracking.riders.phone,
        tracking.riders.email,
        p_address,
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at;
END;
$$ LANGUAGE plpgsql;

-- UPDATE RIDER
CREATE OR REPLACE FUNCTION tracking.sp_update_rider(
    p_rider_id UUID,
    p_phone VARCHAR(50),
    p_email VARCHAR(255),
    p_address VARCHAR(500),
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    rider_type VARCHAR(50),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    guardian_user_id UUID,
    phone VARCHAR(50),
    email VARCHAR(255),
    address VARCHAR(500),
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    -- Check if rider exists
    IF NOT EXISTS (SELECT 1 FROM tracking.riders WHERE tracking.riders.id = p_rider_id) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.riders
    SET
        email = COALESCE(p_email, tracking.riders.email),
        phone = COALESCE(p_phone, tracking.riders.phone),
        status = CASE WHEN p_is_active IS NOT NULL THEN 
                      CASE WHEN p_is_active THEN 'active' ELSE 'inactive' END 
                      ELSE tracking.riders.status 
                 END,
        updated_at = CURRENT_TIMESTAMP
    WHERE tracking.riders.id = p_rider_id
    RETURNING
        tracking.riders.id,
        tracking.riders.company_id,
        NULL::VARCHAR(50),
        tracking.riders.first_name,
        tracking.riders.last_name,
        NULL::UUID,
        tracking.riders.phone,
        tracking.riders.email,
        p_address,
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at;
END;
$$ LANGUAGE plpgsql;
