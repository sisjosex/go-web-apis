CREATE OR REPLACE FUNCTION sales.sp_create_customer(p_tenant_id uuid, p_name character varying, p_email character varying, p_phone_number character varying, p_address character varying, p_city character varying, p_state character varying, p_postal_code character varying, p_country character varying)
 RETURNS TABLE(id uuid, name character varying, email character varying, phone_number character varying, address character varying, city character varying, state character varying, postal_code character varying, country character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_customer_id UUID;
BEGIN
    -- Validate inputs
    IF p_name IS NULL OR TRIM(p_name) = '' THEN
        RAISE EXCEPTION 'customer.invalid-name' USING ERRCODE = 'P0001';
    END IF;

    IF p_email IS NULL OR TRIM(p_email) = '' THEN
        RAISE EXCEPTION 'customer.invalid-email' USING ERRCODE = 'P0001';
    END IF;

    -- Check if email already exists for this tenant
    IF EXISTS (SELECT 1 FROM sales.customers c WHERE LOWER(c.email) = LOWER(TRIM(p_email)) AND c.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'customer.already-exists' USING ERRCODE = 'P0001';
    END IF;

    -- Insert customer
    INSERT INTO sales.customers (
        tenant_id, name, email, phone_number, address, city, state, postal_code, country, status
    ) VALUES (
        p_tenant_id, TRIM(p_name), LOWER(TRIM(p_email)), TRIM(p_phone_number),
        TRIM(p_address), TRIM(p_city), TRIM(p_state), TRIM(p_postal_code), TRIM(p_country), 'active'
    ) RETURNING customers.id INTO v_customer_id;

    -- Return created customer
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
$function$;
