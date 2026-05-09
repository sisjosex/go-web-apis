-- Rider operations with tenant_id validation via transport_companies

CREATE OR REPLACE FUNCTION tracking.sp_create_rider(
    p_tenant_id UUID,
    p_company_id UUID,
    p_rider_type VARCHAR(50),
    p_first_name VARCHAR(100),
    p_last_name VARCHAR(100),
    p_identification_number VARCHAR(50),
    p_phone VARCHAR(20),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(20),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(20),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500)
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_emergency_contact JSONB;
    v_rider_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_emergency_contact := jsonb_build_object(
        'name', p_emergency_contact_name,
        'phone', p_emergency_contact_phone
    );

    INSERT INTO tracking.riders (
        company_id, rider_type, first_name, last_name,
        identification_number, phone, email,
        emergency_contact, status
    )
    VALUES (
        p_company_id,
        COALESCE(NULLIF(TRIM(p_rider_type), ''), 'employee'),
        TRIM(p_first_name),
        TRIM(p_last_name),
        NULLIF(TRIM(p_identification_number), ''),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_email), ''),
        v_emergency_contact,
        'active'
    )
    RETURNING tracking.riders.id INTO v_rider_id;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(r.emergency_contact->>'name' AS VARCHAR),
        CAST(r.emergency_contact->>'phone' AS VARCHAR),
        p_guardian_user_id,
        CAST(p_guardian_name AS VARCHAR),
        CAST(p_guardian_phone AS VARCHAR),
        CAST(p_guardian_email AS VARCHAR),
        CAST(p_address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at
    FROM tracking.riders r
    WHERE r.id = v_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_update_rider(
    p_tenant_id UUID,
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
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_emergency_contact JSONB;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_rider_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_emergency_contact := jsonb_build_object(
        'name', p_emergency_contact_name,
        'phone', p_emergency_contact_phone
    );

    UPDATE tracking.riders r SET
        phone = NULLIF(TRIM(p_phone), ''),
        email = NULLIF(TRIM(p_email), ''),
        emergency_contact = CASE
            WHEN p_emergency_contact_name IS NOT NULL OR p_emergency_contact_phone IS NOT NULL
            THEN v_emergency_contact
            ELSE r.emergency_contact
        END,
        status = CASE
            WHEN p_is_active IS NOT NULL THEN CASE WHEN p_is_active THEN 'active' ELSE 'inactive' END
            ELSE r.status
        END,
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_rider_id;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(r.emergency_contact->>'name' AS VARCHAR),
        CAST(r.emergency_contact->>'phone' AS VARCHAR),
        p_guardian_user_id,
        CAST(p_guardian_name AS VARCHAR),
        CAST(p_guardian_phone AS VARCHAR),
        CAST(p_guardian_email AS VARCHAR),
        CAST(p_address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at
    FROM tracking.riders r
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_riders(
    p_tenant_id UUID,
    p_company_id UUID,
    p_guardian_user_id UUID,
    p_rider_type VARCHAR(50),
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(r.emergency_contact->>'name' AS VARCHAR),
        CAST(r.emergency_contact->>'phone' AS VARCHAR),
        NULL::UUID,
        NULL::VARCHAR,
        NULL::VARCHAR,
        NULL::VARCHAR,
        NULL::VARCHAR,
        (r.status = 'active'),
        r.created_at,
        r.updated_at
    FROM tracking.riders r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_rider_type IS NULL OR r.rider_type = p_rider_type)
      AND (p_is_active IS NULL OR (r.status = 'active') = p_is_active)
    ORDER BY r.created_at DESC;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_get_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_guardian_user_id UUID
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_rider_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(r.emergency_contact->>'name' AS VARCHAR),
        CAST(r.emergency_contact->>'phone' AS VARCHAR),
        NULL::UUID,
        NULL::VARCHAR,
        NULL::VARCHAR,
        NULL::VARCHAR,
        NULL::VARCHAR,
        (r.status = 'active'),
        r.created_at,
        r.updated_at
    FROM tracking.riders r
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_delete_rider(
    p_tenant_id UUID,
    p_rider_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_rider_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.riders r WHERE r.id = p_rider_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;
