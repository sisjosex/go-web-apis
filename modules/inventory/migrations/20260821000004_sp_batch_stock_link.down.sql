-- Revierte INV-014 A1 en inventory: los lotes vuelven a no tocar product_stock.
--
-- Los movimientos ya escritos NO se borran ni se compensan. Son hechos
-- registrados y product_stock se deriva de ellos; deshacerlos dejaría el stock
-- contradiciendo su propio ledger. Bajar esta migración detiene la conexión
-- hacia adelante, no reescribe la historia.
--
-- Las tres funciones vuelven a su cuerpo previo. Firmas y tipos de retorno no
-- cambian: CREATE OR REPLACE.

-- ===========================================================================
-- sp_create_batch — sin movimiento
-- ===========================================================================
CREATE OR REPLACE FUNCTION inventory.sp_create_batch(
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
    v_batch_id UUID := gen_random_uuid();
    v_sku_id   UUID;
    v_status   VARCHAR;
BEGIN
    IF p_sku_id IS NULL AND inventory.fn_requires_sku(p_tenant_id, p_product_id)
    THEN
        RAISE EXCEPTION 'inventory.sku.required' USING ERRCODE = 'P0001';
    END IF;

    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);

    IF p_lot_number IS NULL OR TRIM(p_lot_number) = '' THEN
        RAISE EXCEPTION 'batch.lot-number-required' USING ERRCODE = 'P0001';
    END IF;

    IF p_expiry_date <= CURRENT_DATE THEN
        RAISE EXCEPTION 'batch.expiry-date-must-be-future' USING ERRCODE = 'P0001';
    END IF;

    IF p_purchase_date > CURRENT_DATE THEN
        RAISE EXCEPTION 'batch.purchase-date-cannot-be-future' USING ERRCODE = 'P0001';
    END IF;

    IF p_initial_quantity <= 0 THEN
        RAISE EXCEPTION 'batch.quantity-must-be-positive' USING ERRCODE = 'P0001';
    END IF;

    IF p_unit_cost <= 0 THEN
        RAISE EXCEPTION 'batch.unit-cost-must-be-positive' USING ERRCODE = 'P0001';
    END IF;

    IF p_expiry_date <= CURRENT_DATE THEN
        v_status := 'expired';
    ELSIF p_expiry_date <= CURRENT_DATE + INTERVAL '7 days' THEN
        v_status := 'expiring_soon';
    ELSE
        v_status := 'active';
    END IF;

    INSERT INTO inventory.product_batches(
        id, tenant_id, product_id, sku_id, lot_number, purchase_date, expiry_date,
        unit_cost, initial_quantity, current_quantity, status
    )
    VALUES(
        v_batch_id, p_tenant_id, p_product_id, v_sku_id, TRIM(p_lot_number), p_purchase_date, p_expiry_date,
        p_unit_cost, p_initial_quantity, p_initial_quantity, v_status
    );

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
    'Creates a lot for a SKU. p_sku_id is required when the product tracks stock '
    'by variant and its combinations exist (INV-014 D1), and defaults to the '
    'product SKU otherwise. Same ten columns as before INV-007.';

-- ===========================================================================
-- sp_void_batch — sin movimiento
-- ===========================================================================
CREATE OR REPLACE FUNCTION inventory.sp_void_batch(
    p_tenant_id UUID,
    p_batch_id  UUID
)
RETURNS TABLE(
    id               UUID,
    product_id       UUID,
    sku_id           UUID,
    sku              VARCHAR,
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
    v_status VARCHAR;
BEGIN
    SELECT pb.status
    INTO v_status
    FROM inventory.product_batches pb
    WHERE pb.id = p_batch_id
      AND pb.tenant_id = p_tenant_id
    FOR UPDATE;

    IF v_status IS NULL
    THEN
        RAISE EXCEPTION 'batch.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_status = 'void'
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

    UPDATE inventory.product_batches pb
    SET status     = 'void',
        updated_at = CURRENT_TIMESTAMP
    WHERE pb.id = p_batch_id;

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
$$;

-- ===========================================================================
-- sp_redistribute_product_batches — sin movimiento de existencias
-- ===========================================================================
CREATE OR REPLACE FUNCTION inventory.sp_redistribute_product_batches(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_batch_id   UUID,
    p_targets    JSONB,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE(
    id               UUID,
    product_id       UUID,
    sku_id           UUID,
    sku              VARCHAR,
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
    v_default_sku_id UUID;
    v_batch          inventory.product_batches%ROWTYPE;
    v_target_ids     UUID[];
    v_child_ids      UUID[];
    v_target_count   INT;
    v_total          DECIMAL;
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

    UPDATE inventory.product_batches pb
    SET current_quantity = 0,
        status           = 'depleted',
        updated_at       = CURRENT_TIMESTAMP
    WHERE pb.id = p_batch_id;

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
$$;
