CREATE OR REPLACE FUNCTION inventory.sp_void_batch(p_tenant_id uuid, p_batch_id uuid)
 RETURNS TABLE(id uuid, product_id uuid, sku_id uuid, sku character varying, lot_number character varying, purchase_date date, expiry_date date, unit_cost numeric, initial_quantity numeric, current_quantity numeric, status character varying, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_batch       inventory.product_batches%ROWTYPE;
    v_movement_id UUID;
BEGIN
    SELECT pb.*
    INTO v_batch
    FROM inventory.product_batches pb
    WHERE pb.id = p_batch_id
      AND pb.tenant_id = p_tenant_id
    FOR UPDATE;

    IF v_batch.id IS NULL
    THEN
        RAISE EXCEPTION 'batch.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_batch.status = 'void'
    THEN
        RAISE EXCEPTION 'inventory.batch.voided' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM inventory.batch_movements bm
        WHERE bm.batch_id = p_batch_id
    )
    THEN
        RAISE EXCEPTION 'inventory.batch.has-movements' USING ERRCODE = 'P0001';
    END IF;

    -- 'void' es el único valor autoritativo de esta columna: lo escribe una
    -- acción explícita, no se deduce de una fecha. El FIFO y la venta lo saltan.
    UPDATE inventory.product_batches pb
    SET status     = 'void',
        updated_at = CURRENT_TIMESTAMP
    WHERE pb.id = p_batch_id;

    -- A1 — el lote entró al stock al crearse, así que anularlo tiene que sacarlo.
    -- Sólo lo que le quedaba: lo ya consumido salió por su propia venta. Cantidad
    -- positiva, WASTE la vuelve saliente.
    IF v_batch.current_quantity > 0
    THEN
        SELECT m.movement_id
        INTO v_movement_id
        FROM inventory.sp_record_movement(
            p_tenant_id,
            v_batch.product_id,
            'WASTE',
            v_batch.current_quantity,
            'BATCH',
            p_batch_id,
            NULL,
            'Lote ' || v_batch.lot_number || ' anulado',
            NULL,
            v_batch.sku_id
        ) m;

        UPDATE inventory.inventory_movements im
        SET batch_id = p_batch_id
        WHERE im.id = v_movement_id;
    END IF;

    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.sku_id,
        CAST(s.sku AS VARCHAR),
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(pb.status AS VARCHAR),
        CAST('Batch voided successfully' AS TEXT)
    FROM inventory.product_batches pb
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.id = p_batch_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_void_batch(uuid,uuid) IS 'Da de baja un lote con status = ''void'' — nunca lo borra — y saca del stock lo que todavía tenía, como WASTE sobre su combinación (INV-014 A1). Rechaza un lote ya consumido y uno ya anulado.';
