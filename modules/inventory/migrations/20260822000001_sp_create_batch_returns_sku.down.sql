-- Revierte sp_create_batch a las diez columnas de 20260821000004 (sin sku_id/sku).
DROP FUNCTION IF EXISTS inventory.sp_create_batch(UUID, UUID, VARCHAR, DATE, DATE, DECIMAL, DECIMAL, UUID);

CREATE FUNCTION inventory.sp_create_batch(
    p_tenant_id        UUID,
    p_product_id       UUID,
    p_lot_number       VARCHAR,
    p_purchase_date    DATE,
    p_expiry_date      DATE,
    p_unit_cost        DECIMAL,
    p_initial_quantity DECIMAL,
    p_sku_id           UUID DEFAULT NULL
)
RETURNS TABLE(
    id               UUID,
    product_id       UUID,
    lot_number       VARCHAR,
    purchase_date    DATE,
    expiry_date      DATE,
    unit_cost        DECIMAL,
    initial_quantity DECIMAL,
    current_quantity DECIMAL,
    status           VARCHAR,
    message          TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_batch_id    UUID := gen_random_uuid();
    v_sku_id      UUID;
    v_status      VARCHAR;
    v_movement_id UUID;
BEGIN
    -- INV-014 D1 — la combinación es obligatoria, nunca se asume.
    IF p_sku_id IS NULL AND inventory.fn_requires_sku(p_tenant_id, p_product_id)
    THEN
        RAISE EXCEPTION 'inventory.sku.required' USING ERRCODE = 'P0001';
    END IF;

    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);

    -- Validate lot number is not empty
    IF p_lot_number IS NULL OR TRIM(p_lot_number) = '' THEN
        RAISE EXCEPTION 'batch.lot-number-required' USING ERRCODE = 'P0001';
    END IF;

    -- Validate expiry date is in the future
    IF p_expiry_date <= CURRENT_DATE THEN
        RAISE EXCEPTION 'batch.expiry-date-must-be-future' USING ERRCODE = 'P0001';
    END IF;

    -- Validate purchase date is not in the future
    IF p_purchase_date > CURRENT_DATE THEN
        RAISE EXCEPTION 'batch.purchase-date-cannot-be-future' USING ERRCODE = 'P0001';
    END IF;

    -- Validate quantity is positive
    IF p_initial_quantity <= 0 THEN
        RAISE EXCEPTION 'batch.quantity-must-be-positive' USING ERRCODE = 'P0001';
    END IF;

    -- Validate unit cost is positive
    IF p_unit_cost <= 0 THEN
        RAISE EXCEPTION 'batch.unit-cost-must-be-positive' USING ERRCODE = 'P0001';
    END IF;

    -- Determine status based on expiry date
    IF p_expiry_date <= CURRENT_DATE THEN
        v_status := 'expired';
    ELSIF p_expiry_date <= CURRENT_DATE + INTERVAL '7 days' THEN
        v_status := 'expiring_soon';
    ELSE
        v_status := 'active';
    END IF;

    -- Insert batch using pre-generated UUID (avoids RETURNING INTO column-name ambiguity)
    INSERT INTO inventory.product_batches(
        id, tenant_id, product_id, sku_id, lot_number, purchase_date, expiry_date,
        unit_cost, initial_quantity, current_quantity, status
    )
    VALUES(
        v_batch_id, p_tenant_id, p_product_id, v_sku_id, TRIM(p_lot_number), p_purchase_date, p_expiry_date,
        p_unit_cost, p_initial_quantity, p_initial_quantity, v_status
    );

    -- A1 — la mercancía entró: se registra como PURCHASE sobre la combinación del
    -- lote, y es ese movimiento el que sube product_stock. Cantidad positiva; el
    -- tipo pone el signo.
    SELECT m.movement_id
    INTO v_movement_id
    FROM inventory.sp_record_movement(
        p_tenant_id,
        p_product_id,
        'PURCHASE',
        p_initial_quantity,
        'BATCH',
        v_batch_id,
        p_unit_cost,
        'Lote ' || TRIM(p_lot_number) || ' recibido',
        NULL,
        v_sku_id
    ) m;

    -- El enlace que hace auditable de qué lote salió cada unidad.
    UPDATE inventory.inventory_movements im
    SET batch_id = v_batch_id
    WHERE im.id = v_movement_id;

    -- SELECT back using the pre-generated UUID (fully qualified to avoid any ambiguity)
    RETURN QUERY
    SELECT
        pb.id,
        pb.product_id,
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(pb.status AS VARCHAR),
        'Batch created successfully'::TEXT
    FROM inventory.product_batches pb
    WHERE pb.id = v_batch_id;
END;
$$;
COMMENT ON FUNCTION inventory.sp_create_batch IS
    'Crea un lote para una combinación y registra su entrada al stock como un '
    'movimiento PURCHASE sobre esa misma combinación (INV-014 A1), enlazado por '
    'inventory_movements.batch_id. p_sku_id es obligatorio cuando el producto '
    'controla stock por variante (D1). Mismas diez columnas que antes de INV-007.';
