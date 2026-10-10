CREATE OR REPLACE FUNCTION sales.sp_get_payments(p_tenant_id uuid, p_order_id uuid)
 RETURNS TABLE(id uuid, order_id uuid, customer_id uuid, amount numeric, payment_method character varying, status character varying, reference_number character varying, notes text, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate order belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(p.id AS UUID),
        CAST(p.order_id AS UUID),
        CAST(p.customer_id AS UUID),
        p.amount,
        CAST(p.payment_method AS VARCHAR),
        CAST(p.status AS VARCHAR),
        CAST(p.reference_number AS VARCHAR),
        p.notes,
        p.created_at,
        p.updated_at
    FROM sales.payments p
    WHERE p.order_id = p_order_id
    ORDER BY p.created_at DESC;
END;
$function$;
