-- Remove tenant_id from sales tables (if they exist).
-- Each tenant has its own dedicated database, so tenant_id is redundant.

ALTER TABLE sales.customers
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE sales.sales_orders
    DROP COLUMN IF EXISTS tenant_id;

-- Recreate sp_create_sales_order without tenant_id (replaces v1 and v2)
DROP FUNCTION IF EXISTS sales.sp_create_sales_order(UUID, UUID, VARCHAR, TEXT, DECIMAL);
DROP FUNCTION IF EXISTS sales.sp_create_sales_order(UUID, VARCHAR, TEXT, DECIMAL);
CREATE OR REPLACE FUNCTION sales.sp_create_sales_order(
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
    v_message TEXT := 'Order created successfully';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.customers WHERE id = p_customer_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_order_number := TO_CHAR(CURRENT_DATE, 'YYYY-MM-DD') || '-' ||
                      LPAD(CAST(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP) * 1000 AS BIGINT)::TEXT, 5, '0');

    INSERT INTO sales.sales_orders(
        id, customer_id, order_number, shipping_address, notes,
        discount_amount, status, sub_total, tax_amount, total
    )
    VALUES(
        v_order_id, p_customer_id, v_order_number, p_shipping_address, p_notes,
        COALESCE(p_discount_amount, 0), 'pending', 0, 0, 0
    );

    RETURN QUERY
    SELECT
        v_order_id, v_order_number, p_customer_id,
        'pending'::VARCHAR, 0::DECIMAL, 0::DECIMAL, 0::DECIMAL,
        COALESCE(p_discount_amount, 0), CURRENT_TIMESTAMP, v_message::TEXT;
END;
$$;

-- Recreate sp_get_sales_report without tenant_id
DROP FUNCTION IF EXISTS sales.sp_get_sales_report(UUID, DATE, DATE);
CREATE OR REPLACE FUNCTION sales.sp_get_sales_report(
    p_start_date DATE DEFAULT CURRENT_DATE - INTERVAL '30 days',
    p_end_date DATE DEFAULT CURRENT_DATE
)
RETURNS TABLE(
    metric_name VARCHAR,
    metric_value DECIMAL,
    metric_type VARCHAR
) LANGUAGE plpgsql AS $$
DECLARE
    v_total_orders INT;
    v_total_items INT;
    v_total_revenue DECIMAL;
    v_total_cogs DECIMAL;
    v_total_discount DECIMAL;
    v_avg_order_value DECIMAL;
BEGIN
    SELECT COUNT(*) INTO v_total_orders
    FROM sales.sales_orders
    WHERE DATE(created_at) BETWEEN p_start_date AND p_end_date
      AND status IN ('pending', 'completed');

    SELECT COUNT(*) INTO v_total_items
    FROM sales.order_items oi
    JOIN sales.sales_orders so ON oi.order_id = so.id
    WHERE DATE(so.created_at) BETWEEN p_start_date AND p_end_date
      AND so.status IN ('pending', 'completed');

    SELECT COALESCE(SUM(total), 0) INTO v_total_revenue
    FROM sales.sales_orders
    WHERE DATE(created_at) BETWEEN p_start_date AND p_end_date
      AND status IN ('pending', 'completed');

    SELECT COALESCE(SUM(oba.quantity_assigned * pb.unit_cost), 0) INTO v_total_cogs
    FROM sales.order_batch_assignments oba
    JOIN inventory.product_batches pb ON oba.product_batch_id = pb.id
    JOIN sales.sales_orders so ON oba.order_id = so.id
    WHERE DATE(so.created_at) BETWEEN p_start_date AND p_end_date
      AND so.status IN ('pending', 'completed');

    SELECT COALESCE(SUM(discount_amount), 0) INTO v_total_discount
    FROM sales.sales_orders
    WHERE DATE(created_at) BETWEEN p_start_date AND p_end_date
      AND status IN ('pending', 'completed');

    SELECT CASE
        WHEN v_total_orders = 0 THEN 0
        ELSE v_total_revenue / v_total_orders
    END INTO v_avg_order_value;

    RETURN QUERY
    VALUES
        ('Total Orders'::VARCHAR, v_total_orders::DECIMAL, 'count'::VARCHAR),
        ('Total Items Sold'::VARCHAR, v_total_items::DECIMAL, 'count'::VARCHAR),
        ('Total Revenue'::VARCHAR, v_total_revenue, 'currency'::VARCHAR),
        ('Total COGS'::VARCHAR, v_total_cogs, 'currency'::VARCHAR),
        ('Gross Profit'::VARCHAR, v_total_revenue - v_total_cogs, 'currency'::VARCHAR),
        ('Profit Margin %'::VARCHAR,
         CASE WHEN v_total_revenue = 0 THEN 0
              ELSE ((v_total_revenue - v_total_cogs) / v_total_revenue * 100)
         END, 'percentage'::VARCHAR),
        ('Total Discounts'::VARCHAR, v_total_discount, 'currency'::VARCHAR),
        ('Average Order Value'::VARCHAR, v_avg_order_value, 'currency'::VARCHAR);
END;
$$;
