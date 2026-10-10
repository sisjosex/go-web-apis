CREATE OR REPLACE FUNCTION sales.sp_create_sales_order(p_tenant_id uuid, p_customer_id uuid, p_shipping_address character varying, p_notes text, p_discount_amount numeric DEFAULT 0)
 RETURNS TABLE(id uuid, customer_id uuid, order_number character varying, status character varying, sub_total numeric, tax_amount numeric, total numeric, discount_amount numeric, shipping_address character varying, notes text, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_order_id UUID;
    v_order_number VARCHAR;
    v_sequence TEXT;
BEGIN
    -- Validate customer exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.customers c WHERE c.id = p_customer_id AND c.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Generate order number (YYYY-MM-DD-XXXXX format)
    v_sequence := CAST(NEXTVAL('sales.order_seq') AS TEXT);
    v_order_number := TO_CHAR(CURRENT_DATE, 'YYYY-MM-DD') || '-' ||
                      LPAD(v_sequence, GREATEST(5, LENGTH(v_sequence)), '0');

    -- Insert order with pending status; totals stay at 0 until items are added
    INSERT INTO sales.sales_orders (
        tenant_id, customer_id, order_number, shipping_address, notes, discount_amount
    ) VALUES (
        p_tenant_id, p_customer_id, v_order_number, TRIM(p_shipping_address), p_notes,
        COALESCE(p_discount_amount, 0)
    ) RETURNING sales_orders.id INTO v_order_id;

    -- Return created order
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
    WHERE so.id = v_order_id AND so.tenant_id = p_tenant_id;
END;
$function$;

COMMENT ON FUNCTION sales.sp_create_sales_order(uuid,uuid,character varying,text,numeric) IS 'Creates a pending sales order with a sequence-backed order number scoped to a tenant';
