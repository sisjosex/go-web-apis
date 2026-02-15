-- Fix JSONB ->> operator return type (TEXT to VARCHAR)
-- The ->> operator returns TEXT, but we need VARCHAR for type matching

-- Fix sp_get_rider
CREATE OR REPLACE FUNCTION tracking.sp_get_rider(
    p_rider_id UUID,
    p_guardian_user_id UUID
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    rider_type VARCHAR(50),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    identification_number VARCHAR(100),
    phone VARCHAR(50),
    email VARCHAR(255),
    emergency_contact_name VARCHAR(255),
    emergency_contact_phone VARCHAR(50),
    guardian_user_id UUID,
    guardian_name VARCHAR(255),
    guardian_phone VARCHAR(50),
    guardian_email VARCHAR(255),
    address VARCHAR(500),
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        tracking.riders.id,
        tracking.riders.company_id,
        NULL::VARCHAR(50),
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        (tracking.riders.emergency_contact->>'name')::VARCHAR(255),
        (tracking.riders.emergency_contact->>'phone')::VARCHAR(50),
        NULL::UUID,
        NULL::VARCHAR(255),
        NULL::VARCHAR(50),
        NULL::VARCHAR(255),
        NULL::VARCHAR(500),
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at
    FROM tracking.riders
    WHERE tracking.riders.id = p_rider_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tracking.rider.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Fix sp_list_riders
CREATE OR REPLACE FUNCTION tracking.sp_list_riders(
    p_company_id UUID,
    p_guardian_user_id UUID,
    p_rider_type VARCHAR(50),
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    rider_type VARCHAR(50),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    identification_number VARCHAR(100),
    phone VARCHAR(50),
    email VARCHAR(255),
    emergency_contact_name VARCHAR(255),
    emergency_contact_phone VARCHAR(50),
    guardian_user_id UUID,
    guardian_name VARCHAR(255),
    guardian_phone VARCHAR(50),
    guardian_email VARCHAR(255),
    address VARCHAR(500),
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        tracking.riders.id,
        tracking.riders.company_id,
        NULL::VARCHAR(50),
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        (tracking.riders.emergency_contact->>'name')::VARCHAR(255),
        (tracking.riders.emergency_contact->>'phone')::VARCHAR(50),
        NULL::UUID,
        NULL::VARCHAR(255),
        NULL::VARCHAR(50),
        NULL::VARCHAR(255),
        NULL::VARCHAR(500),
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at
    FROM tracking.riders
    WHERE (p_company_id IS NULL OR tracking.riders.company_id = p_company_id)
      AND (p_is_active IS NULL OR (tracking.riders.status = 'active') = p_is_active)
    ORDER BY tracking.riders.created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- Fix sp_update_rider
CREATE OR REPLACE FUNCTION tracking.sp_update_rider(
    p_rider_id UUID,
    p_phone VARCHAR(50),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(50),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(50),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500),
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    rider_type VARCHAR(50),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    identification_number VARCHAR(100),
    phone VARCHAR(50),
    email VARCHAR(255),
    emergency_contact_name VARCHAR(255),
    emergency_contact_phone VARCHAR(50),
    guardian_user_id UUID,
    guardian_name VARCHAR(255),
    guardian_phone VARCHAR(50),
    guardian_email VARCHAR(255),
    address VARCHAR(500),
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_current_emergency_contact JSONB;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tracking.riders WHERE tracking.riders.id = p_rider_id) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT tracking.riders.emergency_contact INTO v_current_emergency_contact
    FROM tracking.riders 
    WHERE tracking.riders.id = p_rider_id;

    RETURN QUERY
    UPDATE tracking.riders
    SET
        email = COALESCE(p_email, tracking.riders.email),
        phone = COALESCE(p_phone, tracking.riders.phone),
        emergency_contact = CASE 
            WHEN p_emergency_contact_name IS NOT NULL OR p_emergency_contact_phone IS NOT NULL THEN
                jsonb_build_object(
                    'name', COALESCE(p_emergency_contact_name, v_current_emergency_contact->>'name'),
                    'phone', COALESCE(p_emergency_contact_phone, v_current_emergency_contact->>'phone')
                )
            ELSE tracking.riders.emergency_contact
        END,
        status = CASE 
            WHEN p_is_active IS NOT NULL THEN 
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
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        COALESCE(p_emergency_contact_name, (tracking.riders.emergency_contact->>'name')::VARCHAR(255)),
        COALESCE(p_emergency_contact_phone, (tracking.riders.emergency_contact->>'phone')::VARCHAR(50)),
        p_guardian_user_id,
        p_guardian_name,
        p_guardian_phone,
        p_guardian_email,
        p_address,
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at;
END;
$$ LANGUAGE plpgsql;
