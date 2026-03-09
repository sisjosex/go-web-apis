-- Get sales report with totals, product breakdown, and cost analysis
CREATE OR REPLACE FUNCTION sales.sp_get_sales_report(
    p_tenant_id UUID,
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
    -- Total number of orders
    SELECT COUNT(*) INTO v_total_orders
    FROM sales.sales_orders
    WHERE tenant_id = p_tenant_id
        AND DATE(created_at) BETWEEN p_start_date AND p_end_date
        AND status IN ('pending', 'completed');

    -- Total number of items sold
    SELECT COUNT(*) INTO v_total_items
    FROM sales.order_items oi
    JOIN sales.sales_orders so ON oi.order_id = so.id
    WHERE so.tenant_id = p_tenant_id
        AND DATE(so.created_at) BETWEEN p_start_date AND p_end_date
        AND so.status IN ('pending', 'completed');

    -- Total revenue (sum of order totals)
    SELECT COALESCE(SUM(total), 0) INTO v_total_revenue
    FROM sales.sales_orders
    WHERE tenant_id = p_tenant_id
        AND DATE(created_at) BETWEEN p_start_date AND p_end_date
        AND status IN ('pending', 'completed');

    -- Total COGS (sum of cost of assigned batches)
    SELECT COALESCE(SUM(oba.quantity_assigned * pb.unit_cost), 0) INTO v_total_cogs
    FROM sales.order_batch_assignments oba
    JOIN inventory.product_batches pb ON oba.product_batch_id = pb.id
    JOIN sales.sales_orders so ON oba.order_id = so.id
    WHERE so.tenant_id = p_tenant_id
        AND DATE(so.created_at) BETWEEN p_start_date AND p_end_date
        AND so.status IN ('pending', 'completed');

    -- Total discounts applied
    SELECT COALESCE(SUM(discount_amount), 0) INTO v_total_discount
    FROM sales.sales_orders
    WHERE tenant_id = p_tenant_id
        AND DATE(created_at) BETWEEN p_start_date AND p_end_date
        AND status IN ('pending', 'completed');

    -- Average order value
    SELECT CASE 
        WHEN v_total_orders = 0 THEN 0
        ELSE v_total_revenue / v_total_orders
    END INTO v_avg_order_value;

    -- Return all metrics
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
