CREATE OR REPLACE FUNCTION sales.sp_get_payment_by_id(p_tenant_id uuid, p_payment_id uuid)
 RETURNS TABLE(id uuid, order_id uuid, customer_id uuid, amount numeric, payment_method character varying, status character varying, reference_number character varying, notes text, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
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
    JOIN sales.sales_orders o ON o.id = p.order_id
    WHERE p.id = p_payment_id AND o.tenant_id = p_tenant_id;
END;
$function$;
