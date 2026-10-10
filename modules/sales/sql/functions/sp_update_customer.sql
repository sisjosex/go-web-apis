CREATE OR REPLACE FUNCTION sales.sp_update_customer(p_tenant_id uuid, p_customer_id uuid, p_name character varying, p_phone_number character varying, p_address character varying, p_city character varying, p_state character varying, p_postal_code character varying, p_country character varying)
 RETURNS TABLE(id uuid, name character varying, email character varying, phone_number character varying, address character varying, city character varying, state character varying, postal_code character varying, country character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
#variable_conflict use_column
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.customers c WHERE c.id = p_customer_id AND c.tenant_id = p_tenant_id) THEN
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
    WHERE id = p_customer_id AND tenant_id = p_tenant_id;

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
    FROM sales.customers c WHERE c.id = p_customer_id AND c.tenant_id = p_tenant_id;
END;
$function$;
