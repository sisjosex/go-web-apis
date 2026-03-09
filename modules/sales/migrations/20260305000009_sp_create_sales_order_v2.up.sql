-- Create a new sales order with automatic batch assignment and stock reservation
DROP FUNCTION IF EXISTS sales.sp_create_sales_order(UUID, UUID, VARCHAR, TEXT, DECIMAL);
CREATE OR REPLACE FUNCTION sales.sp_create_sales_order(
    p_tenant_id UUID,
    p_customer_id UUID,
    p_shipping_address VARCHAR,
    p_notes TEXT,
    p_discount_amount DECIMAL DEFAULT 0
)
RETURNS TABLE(
    order_id UUID,
    order_number VARCHAR,
    customer_id UUID,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    created_at TIMESTAMP,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_order_id UUID := gen_random_uuid();
    v_order_number VARCHAR;
    v_sub_total DECIMAL := 0;
    v_tax_amount DECIMAL := 0;
    v_total DECIMAL := 0;
    v_message TEXT := 'Order created successfully';
BEGIN
    -- Validate customer exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.customers WHERE id = p_customer_id AND tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Generate order number (YYYY-MM-DD-XXXXX format)
    v_order_number := TO_CHAR(CURRENT_DATE, 'YYYY-MM-DD') || '-' || 
                      LPAD(CAST(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP * 1000) AS BIGINT)::TEXT, 5, '0');

    -- Create order with pending status
    INSERT INTO sales.sales_orders(
        id, tenant_id, customer_id, order_number, shipping_address, notes, 
        discount_amount, status, sub_total, tax_amount, total
    )
    VALUES(
        v_order_id, p_tenant_id, p_customer_id, v_order_number, p_shipping_address, p_notes,
        COALESCE(p_discount_amount, 0), 'pending', 0, 0, 0
    );

    RETURN QUERY
    SELECT 
        v_order_id,
        v_order_number,
        p_customer_id,
        'pending'::VARCHAR,
        v_sub_total,
        v_tax_amount,
        v_total,
        COALESCE(p_discount_amount, 0),
        CURRENT_TIMESTAMP,
        v_message::TEXT;
END;
$$;
