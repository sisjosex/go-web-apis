CREATE OR REPLACE FUNCTION inventory.sp_redistribute_product_batches(p_tenant_id uuid, p_product_id uuid, p_batch_id uuid, p_targets jsonb, p_created_by uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, product_id uuid, sku_id uuid, sku character varying, lot_number character varying, purchase_date date, expiry_date date, unit_cost numeric, initial_quantity numeric, current_quantity numeric, status character varying, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_default_sku_id UUID;
    v_batch          inventory.product_batches%ROWTYPE;
    v_target_ids     UUID[];
    v_child_ids      UUID[];
    v_target_count   INT;
    v_total          DECIMAL;
    v_target         RECORD;
BEGIN
    v_default_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, NULL);

    SELECT pb.*
    INTO v_batch
    FROM inventory.product_batches pb
    WHERE pb.id = p_batch_id
      AND pb.tenant_id = p_tenant_id
      AND pb.product_id = p_product_id
    FOR UPDATE;

    IF v_batch.id IS NULL
    THEN
        RAISE EXCEPTION 'batch.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_batch.sku_id <> v_default_sku_id
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-mismatch'
            USING ERRCODE = 'P0001';
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

    IF p_targets IS NULL OR jsonb_typeof(p_targets) <> 'array'
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-mismatch'
            USING ERRCODE = 'P0001';
    END IF;

    SELECT
        COALESCE(array_agg(DISTINCT (e ->> 'sku_id')::UUID), ARRAY[]::UUID[]),
        COUNT(*),
        COALESCE(SUM((e ->> 'quantity')::DECIMAL), 0)
    INTO v_target_ids, v_target_count, v_total
    FROM jsonb_array_elements(p_targets) e;

    IF v_target_count = 0
       OR v_target_count <> COALESCE(array_length(v_target_ids, 1), 0)
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-mismatch'
            USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM jsonb_array_elements(p_targets) e
        WHERE COALESCE((e ->> 'quantity')::DECIMAL, 0) <= 0
    )
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-mismatch'
            USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM unnest(v_target_ids) AS t(sku_id)
        WHERE NOT EXISTS (
            SELECT 1
            FROM inventory.product_skus s
            WHERE s.id         = t.sku_id
              AND s.product_id = p_product_id
              AND s.tenant_id  = p_tenant_id
              AND NOT s.is_default
        )
    )
    THEN
        RAISE EXCEPTION 'inventory.sku.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_total <> v_batch.current_quantity
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-mismatch'
            USING ERRCODE = 'P0001';
    END IF;

    -- Los ids se capturan aquí en vez de buscarlos después: padre e hijos
    -- comparten lot_number por diseño, así que ningún WHERE los distingue.
    WITH ins AS (
        INSERT INTO inventory.product_batches (
            tenant_id, product_id, sku_id, lot_number, purchase_date,
            expiry_date, unit_cost, initial_quantity, current_quantity, status
        )
        SELECT
            p_tenant_id,
            p_product_id,
            (e ->> 'sku_id')::UUID,
            v_batch.lot_number,
            v_batch.purchase_date,
            v_batch.expiry_date,
            v_batch.unit_cost,
            (e ->> 'quantity')::DECIMAL,
            (e ->> 'quantity')::DECIMAL,
            v_batch.status
        FROM jsonb_array_elements(p_targets) e
        RETURNING product_batches.id
    )
    SELECT array_agg(ins.id) INTO v_child_ids FROM ins;

    -- El padre se vacía, nunca se borra: order_batch_assignments e
    -- inventory_movements pueden apuntarlo, y es la fila de la que cuelga su
    -- historia. 'depleted' es la palabra que ya existe para un lote sin saldo.
    UPDATE inventory.product_batches pb
    SET current_quantity = 0,
        status           = 'depleted',
        updated_at       = CURRENT_TIMESTAMP
    WHERE pb.id = p_batch_id;

    -- A1 — las existencias siguen a los lotes. Salen del bucket una vez y entran
    -- una vez por destino, así el total del producto no cambia.
    PERFORM inventory.sp_record_movement(
        p_tenant_id, p_product_id, 'TRANSFER', v_total,
        'REDISTRIBUTION', p_batch_id, NULL,
        'Lote ' || v_batch.lot_number || ' repartido a las combinaciones',
        p_created_by, v_batch.sku_id, 'OUT'
    );

    FOR v_target IN
        SELECT
            (e ->> 'sku_id')::UUID      AS sku_id,
            (e ->> 'quantity')::DECIMAL AS quantity
        FROM jsonb_array_elements(p_targets) e
    LOOP
        PERFORM inventory.sp_record_movement(
            p_tenant_id, p_product_id, 'TRANSFER', v_target.quantity,
            'REDISTRIBUTION', p_batch_id, NULL,
            'Lote ' || v_batch.lot_number || ' recibido del SKU por defecto',
            p_created_by, v_target.sku_id, 'IN'
        );
    END LOOP;

    RETURN QUERY
    SELECT
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
        CAST('Batch redistributed' AS TEXT)
    FROM inventory.product_batches pb
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.id = ANY (v_child_ids)
    ORDER BY s.sku;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_redistribute_product_batches(uuid,uuid,uuid,jsonb,uuid) IS 'Reparte un lote del SKU por defecto entre combinaciones generadas, en una sola transacción (INV-014 D2). Los importes deben sumar exactamente el saldo del lote. El original se vacía y se conserva, los hijos heredan número de lote, fechas y costo, y un lote ya consumido se rechaza en vez de partirse. Las existencias siguen a los lotes mediante TRANSFER saliente del bucket y entrante por destino (A1), así que el total del producto no cambia.';
