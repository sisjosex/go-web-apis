-- INV-014 A1 — los lotes alimentan el stock.
--
-- Hasta aquí lotes y existencias eran dos libros que no se tocaban:
-- sp_create_batch escribía product_batches y nada más, y sólo sp_record_movement
-- escribía product_stock. Un lote de 10 dejaba la pestaña de Existencias en 0.
--
-- La regla que se establece: sp_record_movement es la ÚNICA puerta a
-- product_stock. Los lotes no escriben existencias por su cuenta, pasan por esa
-- puerta — así cualquier integración futura hereda la conexión con sólo usarla.
--
-- Dos convenciones de INV-013 que estas llamadas respetan:
--   * la cantidad va SIEMPRE positiva; el tipo decide el signo
--     (fn_movement_direction: PURCHASE +1, WASTE -1, SALE -1)
--   * unit_cost sólo viaja en PURCHASE y PRODUCTION; en los demás se fuerza a
--     NULL, así que no se envía
--
-- El movimiento queda enlazado al lote por inventory_movements.batch_id, que la
-- tabla ya tenía y nadie llenaba.
--
-- Ninguna de las tres funciones cambia firma ni tipo de retorno: CREATE OR REPLACE.

-- ===========================================================================
-- sp_create_batch — el lote entra al stock
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

-- ===========================================================================
-- sp_void_batch — anular devuelve al stock lo que el lote todavía tenía
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
$$;
COMMENT ON FUNCTION inventory.sp_void_batch IS
    'Da de baja un lote con status = ''void'' — nunca lo borra — y saca del stock '
    'lo que todavía tenía, como WASTE sobre su combinación (INV-014 A1). Rechaza '
    'un lote ya consumido y uno ya anulado.';

-- ===========================================================================
-- sp_redistribute_product_batches — el split mueve también las existencias
-- ===========================================================================
-- Ahora que el lote SÍ aportó stock al crearse, repartirlo tiene que repartir
-- también las existencias: un TRANSFER saliente del bucket y uno entrante por
-- cada combinación destino. TRANSFER es de los dos tipos que no deciden su
-- dirección, así que cada llamada la dice explícitamente (INV-013 D1).
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
$$;
COMMENT ON FUNCTION inventory.sp_redistribute_product_batches IS
    'Reparte un lote del SKU por defecto entre combinaciones generadas, en una '
    'sola transacción (INV-014 D2). Los importes deben sumar exactamente el saldo '
    'del lote. El original se vacía y se conserva, los hijos heredan número de '
    'lote, fechas y costo, y un lote ya consumido se rechaza en vez de partirse. '
    'Las existencias siguen a los lotes mediante TRANSFER saliente del bucket y '
    'entrante por destino (A1), así que el total del producto no cambia.';
