-- Company operations with tenant_id isolation

CREATE OR REPLACE FUNCTION tracking.sp_create_company(
    p_tenant_id UUID,
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
    IF EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.registration_number = p_registration_number
          AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.registration-number.already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.transport_companies (
        tenant_id, name, email, phone, address, city, country, registration_number, status
    )
    VALUES (
        p_tenant_id,
        TRIM(p_name),
        NULLIF(TRIM(p_email), ''),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_address), ''),
        NULLIF(TRIM(p_city), ''),
        NULLIF(TRIM(p_country), ''),
        TRIM(p_registration_number),
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING
        tracking.transport_companies.id,
        tracking.transport_companies.tenant_id,
        CAST(tracking.transport_companies.name AS VARCHAR),
        CAST(tracking.transport_companies.email AS VARCHAR),
        CAST(tracking.transport_companies.phone AS VARCHAR),
        tracking.transport_companies.address,
        CAST(tracking.transport_companies.city AS VARCHAR),
        CAST(tracking.transport_companies.country AS VARCHAR),
        CAST(tracking.transport_companies.registration_number AS VARCHAR),
        CAST(tracking.transport_companies.status AS VARCHAR),
        tracking.transport_companies.created_at,
        tracking.transport_companies.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_update_company(
    p_tenant_id UUID,
    p_company_id UUID,
    p_name VARCHAR(255),
    p_email VARCHAR(255),
    p_phone VARCHAR(20),
    p_address TEXT,
    p_city VARCHAR(100),
    p_country VARCHAR(100)
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
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.transport_companies tc SET
        name = COALESCE(NULLIF(TRIM(p_name), ''), tc.name),
        email = NULLIF(TRIM(p_email), ''),
        phone = NULLIF(TRIM(p_phone), ''),
        address = NULLIF(TRIM(p_address), ''),
        city = NULLIF(TRIM(p_city), ''),
        country = NULLIF(TRIM(p_country), ''),
        updated_at = CURRENT_TIMESTAMP
    WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    RETURNING
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
        tc.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_companies(
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

CREATE OR REPLACE FUNCTION tracking.sp_get_company(
    p_tenant_id UUID,
    p_company_id UUID
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
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

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
    WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_delete_company(
    p_tenant_id UUID,
    p_company_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.transport_companies tc
    WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id;

    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;
