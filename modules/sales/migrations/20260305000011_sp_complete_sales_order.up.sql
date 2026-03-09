-- Complete a sales order - transition from pending/reserved to completed and consume inventory
CREATE OR REPLACE FUNCTION sales.sp_complete_sales_order(
    p_order_id UUID
)
RETURNS TABLE(
    order_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    total DECIMAL,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_order_record RECORD;
    v_batch_record RECORD;
    v_total_consumed DECIMAL;
    v_current_batch_qty DECIMAL;
BEGIN
    -- Get order details
    SELECT id, order_number, status, total
    INTO v_order_record
    FROM sales.sales_orders
    WHERE id = p_order_id;

    IF v_order_record IS NULL THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Only complete if pending or reserved
    IF v_order_record.status NOT IN ('pending', 'reserved') THEN
        RAISE EXCEPTION 'sales-order.cannot-modify' USING ERRCODE = 'P0001';
    END IF;

    -- For each batch assignment, reduce inventory
    FOR v_batch_record IN
        SELECT oba.product_batch_id, oba.quantity_assigned
        FROM sales.order_batch_assignments oba
        WHERE oba.order_id = p_order_id
    LOOP
        -- Get current batch quantity
        SELECT current_quantity INTO v_current_batch_qty
        FROM inventory.product_batches
        WHERE id = v_batch_record.product_batch_id;

        IF v_current_batch_qty IS NULL THEN
            RAISE EXCEPTION 'product-batch.not-found' USING ERRCODE = 'P0001';
        END IF;

        IF v_current_batch_qty < v_batch_record.quantity_assigned THEN
            RAISE EXCEPTION 'batch.insufficient-quantity' USING ERRCODE = 'P0001';
        END IF;

        -- Reduce batch current_quantity
        UPDATE inventory.product_batches
        SET current_quantity = current_quantity - v_batch_record.quantity_assigned
        WHERE id = v_batch_record.product_batch_id;
    END LOOP;

    -- Update order status to completed
    UPDATE sales.sales_orders
    SET status = 'completed'
    WHERE id = p_order_id;

    RETURN QUERY
    SELECT
        p_order_id,
        v_order_record.order_number,
        'completed'::VARCHAR,
        v_order_record.total,
        'Order completed and inventory consumed'::TEXT;
END;
$$;
