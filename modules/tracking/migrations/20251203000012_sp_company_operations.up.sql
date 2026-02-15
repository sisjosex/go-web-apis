-- CREATE COMPANY
CREATE OR REPLACE FUNCTION tracking.sp_create_company(
    p_name VARCHAR(255),
    p_email VARCHAR(255),
    p_phone VARCHAR(20),
    p_address TEXT,
    p_city VARCHAR(100),
    p_country VARCHAR(100),
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
    -- Check if registration_number already exists
    IF EXISTS (
        SELECT 1 FROM tracking.transport_companies 
        WHERE transport_companies.registration_number = p_registration_number
    ) THEN
        RAISE EXCEPTION 'tracking.company.already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.transport_companies (
        name, 
        email, 
        phone, 
        address, 
        city, 
        country, 
        registration_number, 
        status
    )
    VALUES (
        p_name, 
        NULLIF(p_email, ''), 
        NULLIF(p_phone, ''), 
        NULLIF(p_address, ''), 
        NULLIF(p_city, ''), 
        NULLIF(p_country, ''), 
        p_registration_number, 
        COALESCE(NULLIF(p_status, ''), 'active')
    )
    RETURNING 
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
        updated_at;
END;
$$ LANGUAGE plpgsql;

-- UPDATE COMPANY
CREATE OR REPLACE FUNCTION tracking.sp_update_company(
    p_company_id UUID,
    p_name VARCHAR(255),
    p_email VARCHAR(255),
    p_phone VARCHAR(20),
    p_address TEXT,
    p_city VARCHAR(100),
    p_country VARCHAR(100)
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
    UPDATE tracking.transport_companies t
    SET 
        name = COALESCE(p_name, t.name),
        email = COALESCE(p_email, t.email),
        phone = COALESCE(p_phone, t.phone),
        address = COALESCE(p_address, t.address),
        city = COALESCE(p_city, t.city),
        country = COALESCE(p_country, t.country),
        updated_at = CURRENT_TIMESTAMP
    WHERE t.id = p_company_id
    RETURNING 
        t.id,
        t.name,
        t.email,
        t.phone,
        t.address,
        t.city,
        t.country,
        t.registration_number,
        t.status,
        t.created_at,
        t.updated_at;
END;
$$ LANGUAGE plpgsql;
