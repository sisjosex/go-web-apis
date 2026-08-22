-- Revierte INV-014 A1 en ventas: completar un pedido vuelve a bajar sólo el
-- saldo del lote, sin movimiento SALE y sin fila en batch_movements.
--
-- Lo ya escrito no se borra: los movimientos son hechos y product_stock se
-- deriva de ellos, y las filas de batch_movements son el COGS imputado. Bajar
-- esta migración detiene la conexión hacia adelante, no reescribe la historia.

CREATE OR REPLACE FUNCTION sales.sp_complete_sales_order(
    p_tenant_id UUID,
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
    v_current_batch_qty DECIMAL;
BEGIN
    SELECT o.id, o.order_number, o.status, o.total
    INTO v_order_record
    FROM sales.sales_orders o
    WHERE o.id = p_order_id AND o.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_order_record.status NOT IN ('pending', 'reserved') THEN
        RAISE EXCEPTION 'sales-order.cannot-modify' USING ERRCODE = 'P0001';
    END IF;

    FOR v_batch_record IN
        SELECT oba.product_batch_id, oba.quantity_assigned
        FROM sales.order_batch_assignments oba
        WHERE oba.order_id = p_order_id
    LOOP
        SELECT pb.current_quantity INTO v_current_batch_qty
        FROM inventory.product_batches pb
        WHERE pb.id = v_batch_record.product_batch_id;

        IF v_current_batch_qty IS NULL THEN
            RAISE EXCEPTION 'product-batch.not-found' USING ERRCODE = 'P0001';
        END IF;

        IF v_current_batch_qty < v_batch_record.quantity_assigned THEN
            RAISE EXCEPTION 'batch.insufficient-quantity' USING ERRCODE = 'P0001';
        END IF;

        UPDATE inventory.product_batches
        SET current_quantity = current_quantity - v_batch_record.quantity_assigned
        WHERE id = v_batch_record.product_batch_id;
    END LOOP;

    UPDATE sales.sales_orders
    SET status = 'completed'
    WHERE id = p_order_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        p_order_id,
        v_order_record.order_number,
        'completed'::VARCHAR,
        v_order_record.total,
        'Order completed and inventory consumed'::TEXT;
END;
$$;
