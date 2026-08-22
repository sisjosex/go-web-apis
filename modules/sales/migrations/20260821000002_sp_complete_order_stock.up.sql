-- INV-014 A1 (mitad de ventas) — completar una orden descuenta existencias.
--
-- Hasta aquí completar una orden bajaba product_batches.current_quantity y nada
-- más: nadie escribía un movimiento SALE, así que product_stock no se movía al
-- vender. Con A1 el lote SUBE el stock al crearse, de modo que si la venta no lo
-- bajara el stock se inflaría en cada pedido. Las dos mitades van juntas.
--
-- Además se escribe por fin inventory.batch_movements. La tabla existe desde el
-- principio con sus FKs y sus índices, y nada la llenaba nunca; es la que guarda
-- el COGS imputado a cada lote, y es la que hace que el guardia
-- inventory.batch.has-movements de INV-014 D3 se dispare de verdad — hasta ahora
-- un lote ya vendido seguía siendo corregible y anulable por API.
--
-- El movimiento va por sp_record_movement, la única puerta a product_stock:
-- cantidad positiva, el tipo SALE pone el signo (INV-013 D1), y unit_cost no
-- viaja porque sólo PURCHASE y PRODUCTION lo llevan.
--
-- Firma y tipo de retorno no cambian: CREATE OR REPLACE.

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
    v_movement_id UUID;
BEGIN
    -- Get order details (validate tenant)
    SELECT o.id, o.order_number, o.status, o.total
    INTO v_order_record
    FROM sales.sales_orders o
    WHERE o.id = p_order_id AND o.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Only complete if pending or reserved
    IF v_order_record.status NOT IN ('pending', 'reserved') THEN
        RAISE EXCEPTION 'sales-order.cannot-modify' USING ERRCODE = 'P0001';
    END IF;

    -- For each batch assignment, reduce inventory
    FOR v_batch_record IN
        SELECT
            oba.product_batch_id,
            oba.quantity_assigned,
            pb.product_id,
            pb.sku_id,
            pb.unit_cost,
            pb.lot_number
        FROM sales.order_batch_assignments oba
        JOIN inventory.product_batches pb ON pb.id = oba.product_batch_id
        WHERE oba.order_id = p_order_id
        ORDER BY oba.product_batch_id
    LOOP
        -- Get current batch quantity
        SELECT pb.current_quantity INTO v_current_batch_qty
        FROM inventory.product_batches pb
        WHERE pb.id = v_batch_record.product_batch_id
        FOR UPDATE;

        IF v_current_batch_qty IS NULL THEN
            RAISE EXCEPTION 'product-batch.not-found' USING ERRCODE = 'P0001';
        END IF;

        IF v_current_batch_qty < v_batch_record.quantity_assigned THEN
            RAISE EXCEPTION 'batch.insufficient-quantity' USING ERRCODE = 'P0001';
        END IF;

        -- Reduce batch current_quantity
        UPDATE inventory.product_batches
        SET current_quantity = current_quantity - v_batch_record.quantity_assigned,
            updated_at       = CURRENT_TIMESTAMP
        WHERE id = v_batch_record.product_batch_id;

        -- A1 — el stock sale por la misma puerta que entró.
        SELECT m.movement_id
        INTO v_movement_id
        FROM inventory.sp_record_movement(
            p_tenant_id,
            v_batch_record.product_id,
            'SALE',
            v_batch_record.quantity_assigned,
            'SALES_ORDER',
            p_order_id,
            NULL,
            'Pedido ' || v_order_record.order_number || ' — lote '
                || v_batch_record.lot_number,
            NULL,
            v_batch_record.sku_id
        ) m;

        UPDATE inventory.inventory_movements im
        SET batch_id = v_batch_record.product_batch_id
        WHERE im.id = v_movement_id;

        -- A1 — el consumo del lote queda registrado con el costo que se imputa.
        -- Es la fila que INV-014 D3 lee para negarse a corregir un lote vendido.
        INSERT INTO inventory.batch_movements (
            tenant_id, batch_id, movement_id, quantity_consumed, cost_of_goods_sold
        ) VALUES (
            p_tenant_id,
            v_batch_record.product_batch_id,
            v_movement_id,
            v_batch_record.quantity_assigned,
            v_batch_record.quantity_assigned * v_batch_record.unit_cost
        );
    END LOOP;

    -- Update order status to completed
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
COMMENT ON FUNCTION sales.sp_complete_sales_order(UUID, UUID) IS
    'Completa un pedido y consume el inventario asignado: baja el saldo de cada '
    'lote, descuenta las existencias con un movimiento SALE sobre la combinación '
    'del lote y registra el consumo en inventory.batch_movements con su COGS '
    '(INV-014 A1). Las tres cosas van juntas porque el lote sube el stock al '
    'crearse; descontar sólo el lote lo dejaría inflado.';
