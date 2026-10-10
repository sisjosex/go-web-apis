CREATE OR REPLACE FUNCTION sales.sp_get_all_customers(p_tenant_id uuid, p_limit integer DEFAULT 20, p_offset integer DEFAULT 0)
 RETURNS TABLE(id uuid, name character varying, email character varying, phone_number character varying, address character varying, city character varying, state character varying, postal_code character varying, country character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
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
    FROM sales.customers c
    WHERE c.tenant_id = p_tenant_id AND c.status = 'active'
    ORDER BY c.created_at DESC
    LIMIT p_limit OFFSET p_offset;
END;
$function$;
