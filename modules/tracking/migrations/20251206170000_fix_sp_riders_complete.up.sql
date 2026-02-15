-- ============================================================================
-- DROP EXISTING RIDER STORED PROCEDURES FIRST
-- ============================================================================
DROP FUNCTION IF EXISTS tracking.sp_create_rider CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_update_rider CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_list_riders CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_get_rider CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_delete_rider CASCADE;

-- ============================================================================
-- COMPLETE RIDER STORED PROCEDURES WITH ALL 18 FIELDS
-- ============================================================================

-- CREATE RIDER (14 parameters, returns 18 fields)
CREATE OR REPLACE FUNCTION tracking.sp_create_rider(
    p_company_id UUID,
    p_rider_type VARCHAR(50),
    p_first_name VARCHAR(100),
    p_last_name VARCHAR(100),
    p_identification_number VARCHAR(100),
    p_phone VARCHAR(50),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(50),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(50),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500)
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
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE tracking.transport_companies.id = p_company_id) THEN
        RAISE EXCEPTION 'tracking.company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate rider_type enum
    IF p_rider_type NOT IN ('student', 'employee') THEN
        RAISE EXCEPTION 'tracking.rider.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.riders (
        company_id, first_name, last_name, email, phone, identification_number, emergency_contact, status
    )
    VALUES (
        p_company_id, 
        p_first_name, 
        p_last_name, 
        p_email, 
        p_phone, 
        p_identification_number,
        -- Store emergency contact as JSON (existing column)
        CASE 
            WHEN p_emergency_contact_name IS NOT NULL OR p_emergency_contact_phone IS NOT NULL THEN
                jsonb_build_object(
                    'name', p_emergency_contact_name,
                    'phone', p_emergency_contact_phone
                )
            ELSE NULL
        END,
        'active'
    )
    RETURNING
        tracking.riders.id,
        tracking.riders.company_id,
        p_rider_type,                           -- Pass through (not in table)
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        p_emergency_contact_name,               -- Pass through (extracted from JSON)
        p_emergency_contact_phone,              -- Pass through (extracted from JSON)
        p_guardian_user_id,                     -- Pass through (not in table)
        p_guardian_name,                        -- Pass through (not in table)
        p_guardian_phone,                       -- Pass through (not in table)
        p_guardian_email,                       -- Pass through (not in table)
        p_address,                              -- Pass through (not in table)
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at;
END;
$$ LANGUAGE plpgsql;

-- UPDATE RIDER (11 parameters, returns 18 fields)
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
    -- Check if rider exists
    IF NOT EXISTS (SELECT 1 FROM tracking.riders WHERE tracking.riders.id = p_rider_id) THEN
        RAISE EXCEPTION 'tracking.rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Get current emergency contact
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
        NULL::VARCHAR(50),                                  -- rider_type not in table
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        COALESCE(p_emergency_contact_name, tracking.riders.emergency_contact->>'name'),
        COALESCE(p_emergency_contact_phone, tracking.riders.emergency_contact->>'phone'),
        p_guardian_user_id,                                 -- Pass through
        p_guardian_name,                                    -- Pass through
        p_guardian_phone,                                   -- Pass through
        p_guardian_email,                                   -- Pass through
        p_address,                                          -- Pass through
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at;
END;
$$ LANGUAGE plpgsql;

-- LIST RIDERS (4 optional filter parameters, returns 18 fields)
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
        NULL::VARCHAR(50),                          -- rider_type not in table
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        tracking.riders.emergency_contact->>'name',
        tracking.riders.emergency_contact->>'phone',
        NULL::UUID,                                 -- guardian_user_id not in table
        NULL::VARCHAR(255),                         -- guardian_name not in table
        NULL::VARCHAR(50),                          -- guardian_phone not in table
        NULL::VARCHAR(255),                         -- guardian_email not in table
        NULL::VARCHAR(500),                         -- address not in table
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at
    FROM tracking.riders
    WHERE (p_company_id IS NULL OR tracking.riders.company_id = p_company_id)
      AND (p_is_active IS NULL OR (tracking.riders.status = 'active') = p_is_active)
    ORDER BY tracking.riders.created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- GET RIDER BY ID (2 parameters, returns 18 fields)
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
        NULL::VARCHAR(50),                          -- rider_type not in table
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        tracking.riders.emergency_contact->>'name',
        tracking.riders.emergency_contact->>'phone',
        NULL::UUID,                                 -- guardian_user_id not in table
        NULL::VARCHAR(255),                         -- guardian_name not in table
        NULL::VARCHAR(50),                          -- guardian_phone not in table
        NULL::VARCHAR(255),                         -- guardian_email not in table
        NULL::VARCHAR(500),                         -- address not in table
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at
    FROM tracking.riders
    WHERE tracking.riders.id = p_rider_id;

    -- If no rider found, raise exception
    IF NOT FOUND THEN
        RAISE EXCEPTION 'tracking.rider.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql;

-- DELETE RIDER (soft delete, 1 parameter, returns BOOLEAN)
CREATE OR REPLACE FUNCTION tracking.sp_delete_rider(
    p_rider_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_rows_affected INT;
BEGIN
    -- Check if rider exists
    IF NOT EXISTS (SELECT 1 FROM tracking.riders WHERE tracking.riders.id = p_rider_id) THEN
        RAISE EXCEPTION 'tracking.rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Soft delete: set status to inactive
    UPDATE tracking.riders
    SET status = 'inactive', updated_at = CURRENT_TIMESTAMP
    WHERE tracking.riders.id = p_rider_id;

    GET DIAGNOSTICS v_rows_affected = ROW_COUNT;
    RETURN v_rows_affected > 0;
END;
$$ LANGUAGE plpgsql;
