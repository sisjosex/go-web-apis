CREATE OR REPLACE FUNCTION sales.sp_get_return(p_tenant_id uuid, p_return_id uuid)
 RETURNS TABLE(id uuid, order_id uuid, customer_id uuid, return_number character varying, total_amount numeric, reason text, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        CAST(r.id AS UUID),
        CAST(r.order_id AS UUID),
        CAST(r.customer_id AS UUID),
        CAST(r.return_number AS VARCHAR),
        r.total_amount,
        CAST(r.reason AS TEXT),
        CAST(r.status AS VARCHAR),
        r.created_at,
        r.updated_at
    FROM sales.returns r
    JOIN sales.sales_orders o ON o.id = r.order_id
    WHERE r.id = p_return_id AND o.tenant_id = p_tenant_id;
END;
$function$;
