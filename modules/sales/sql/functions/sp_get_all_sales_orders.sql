CREATE OR REPLACE FUNCTION sales.sp_get_all_sales_orders(p_tenant_id uuid, p_limit integer DEFAULT 20, p_offset integer DEFAULT 0)
 RETURNS TABLE(id uuid, customer_id uuid, order_number character varying, status character varying, sub_total numeric, tax_amount numeric, total numeric, discount_amount numeric, shipping_address character varying, notes text, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so
    WHERE so.tenant_id = p_tenant_id
    ORDER BY so.created_at DESC
    LIMIT p_limit OFFSET p_offset;
END;
$function$;
