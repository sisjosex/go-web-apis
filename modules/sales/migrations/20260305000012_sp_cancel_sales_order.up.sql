-- Cancel a sales order - revert status and release batch assignments
CREATE OR REPLACE FUNCTION sales.sp_cancel_sales_order(
    p_order_id UUID
)
RETURNS TABLE(
    order_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_order_record RECORD;
BEGIN
    -- Get order details
    SELECT id, order_number, status
    INTO v_order_record
    FROM sales.sales_orders
    WHERE id = p_order_id;

    IF v_order_record IS NULL THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Cannot cancel if already completed
    IF v_order_record.status = 'completed' THEN
        RAISE EXCEPTION 'sales-order.cannot-modify' USING ERRCODE = 'P0001';
    END IF;

    -- Delete all batch assignments (reverting the reservations)
    DELETE FROM sales.order_batch_assignments WHERE order_id = p_order_id;

    -- Update order status to cancelled
    UPDATE sales.sales_orders
    SET status = 'cancelled'
    WHERE id = p_order_id;

    RETURN QUERY
    SELECT
        p_order_id,
        v_order_record.order_number,
        'cancelled'::VARCHAR,
        'Order cancelled successfully'::TEXT;
END;
$$;
