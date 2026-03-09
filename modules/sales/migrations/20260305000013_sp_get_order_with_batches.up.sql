-- Get a sales order with all batch assignments and inventory details
CREATE OR REPLACE FUNCTION sales.sp_get_order_with_batches(
    p_order_id UUID
)
RETURNS TABLE(
    order_id UUID,
    order_number VARCHAR,
    customer_id UUID,
    customer_name VARCHAR,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    shipping_address VARCHAR,
    item_count INT,
    batch_count INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
DECLARE
    v_item_count INT;
    v_batch_count INT;
BEGIN
    -- Count items and batches
    SELECT COUNT(*) INTO v_item_count FROM sales.order_items WHERE order_id = p_order_id;
    SELECT COUNT(*) INTO v_batch_count FROM sales.order_batch_assignments WHERE order_id = p_order_id;

    RETURN QUERY
    SELECT
        so.id,
        so.order_number,
        so.customer_id,
        c.name,
        so.status,
        so.sub_total,
        so.tax_amount,
        so.total,
        so.discount_amount,
        so.shipping_address,
        v_item_count,
        v_batch_count,
        so.created_at,
        so.updated_at
    FROM sales.sales_orders so
    LEFT JOIN sales.customers c ON c.id = so.customer_id
    WHERE so.id = p_order_id;
END;
$$;
