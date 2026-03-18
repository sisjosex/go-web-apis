-- Fix ambiguous column references in sales stored procedures.
-- Functions with RETURNS TABLE declare output variables with the same names as
-- table columns, causing PostgreSQL to raise "column reference is ambiguous"
-- when unqualified column names are used inside the function body.

-- Fix sp_get_customer_by_id: qualify 'id' with table alias
CREATE OR REPLACE FUNCTION sales.sp_get_customer_by_id(p_customer_id UUID)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    email VARCHAR,
    phone_number VARCHAR,
    address VARCHAR,
    city VARCHAR,
    state VARCHAR,
    postal_code VARCHAR,
    country VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.customers c2 WHERE c2.id = p_customer_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(c.id AS UUID),
        CAST(c.name AS VARCHAR),
        CAST(c.email AS VARCHAR),
        CAST(c.phone_number AS VARCHAR),
        CAST(c.address AS VARCHAR),
        CAST(c.city AS VARCHAR),
        CAST(c.state AS VARCHAR),
        CAST(c.postal_code AS VARCHAR),
        CAST(c.country AS VARCHAR),
        CAST(c.status AS VARCHAR),
        CAST(c.created_at AS TIMESTAMP),
        CAST(c.updated_at AS TIMESTAMP)
    FROM sales.customers c WHERE c.id = p_customer_id;
END;
$$;

-- Fix sp_update_customer: qualify 'id' with table alias
CREATE OR REPLACE FUNCTION sales.sp_update_customer(
    p_customer_id UUID,
    p_name VARCHAR,
    p_phone_number VARCHAR,
    p_address VARCHAR,
    p_city VARCHAR,
    p_state VARCHAR,
    p_postal_code VARCHAR,
    p_country VARCHAR
)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    email VARCHAR,
    phone_number VARCHAR,
    address VARCHAR,
    city VARCHAR,
    state VARCHAR,
    postal_code VARCHAR,
    country VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.customers c2 WHERE c2.id = p_customer_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE sales.customers SET
        name = COALESCE(p_name, name),
        phone_number = COALESCE(p_phone_number, phone_number),
        address = COALESCE(p_address, address),
        city = COALESCE(p_city, city),
        state = COALESCE(p_state, state),
        postal_code = COALESCE(p_postal_code, postal_code),
        country = COALESCE(p_country, country)
    WHERE customers.id = p_customer_id;

    RETURN QUERY
    SELECT
        CAST(c.id AS UUID),
        CAST(c.name AS VARCHAR),
        CAST(c.email AS VARCHAR),
        CAST(c.phone_number AS VARCHAR),
        CAST(c.address AS VARCHAR),
        CAST(c.city AS VARCHAR),
        CAST(c.state AS VARCHAR),
        CAST(c.postal_code AS VARCHAR),
        CAST(c.country AS VARCHAR),
        CAST(c.status AS VARCHAR),
        CAST(c.created_at AS TIMESTAMP),
        CAST(c.updated_at AS TIMESTAMP)
    FROM sales.customers c WHERE c.id = p_customer_id;
END;
$$;

-- Fix sp_create_customer: qualify 'email' with table alias to avoid RETURNS TABLE ambiguity
CREATE OR REPLACE FUNCTION sales.sp_create_customer(
    p_name VARCHAR,
    p_email VARCHAR,
    p_phone_number VARCHAR,
    p_address VARCHAR,
    p_city VARCHAR,
    p_state VARCHAR,
    p_postal_code VARCHAR,
    p_country VARCHAR
)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    email VARCHAR,
    phone_number VARCHAR,
    address VARCHAR,
    city VARCHAR,
    state VARCHAR,
    postal_code VARCHAR,
    country VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
DECLARE
    v_customer_id UUID;
BEGIN
    IF p_name IS NULL OR TRIM(p_name) = '' THEN
        RAISE EXCEPTION 'customer.invalid-name' USING ERRCODE = 'P0001';
    END IF;

    IF p_email IS NULL OR TRIM(p_email) = '' THEN
        RAISE EXCEPTION 'customer.invalid-email' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (SELECT 1 FROM sales.customers c2 WHERE LOWER(c2.email) = LOWER(TRIM(p_email))) THEN
        RAISE EXCEPTION 'customer.already-exists' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO sales.customers (
        name, email, phone_number, address, city, state, postal_code, country, status
    ) VALUES (
        TRIM(p_name), LOWER(TRIM(p_email)), TRIM(p_phone_number),
        TRIM(p_address), TRIM(p_city), TRIM(p_state), TRIM(p_postal_code), TRIM(p_country), 'active'
    ) RETURNING customers.id INTO v_customer_id;

    RETURN QUERY
    SELECT
        CAST(c.id AS UUID),
        CAST(c.name AS VARCHAR),
        CAST(c.email AS VARCHAR),
        CAST(c.phone_number AS VARCHAR),
        CAST(c.address AS VARCHAR),
        CAST(c.city AS VARCHAR),
        CAST(c.state AS VARCHAR),
        CAST(c.postal_code AS VARCHAR),
        CAST(c.country AS VARCHAR),
        CAST(c.status AS VARCHAR),
        CAST(c.created_at AS TIMESTAMP),
        CAST(c.updated_at AS TIMESTAMP)
    FROM sales.customers c WHERE c.id = v_customer_id;
END;
$$;
