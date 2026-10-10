CREATE OR REPLACE FUNCTION sales.sp_get_returns_by_order(p_tenant_id uuid, p_order_id uuid)
 RETURNS TABLE(id uuid, order_id uuid, customer_id uuid, return_number character varying, total_amount numeric, reason text, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate order belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

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
    WHERE r.order_id = p_order_id
    ORDER BY r.created_at DESC;
END;
$function$;
