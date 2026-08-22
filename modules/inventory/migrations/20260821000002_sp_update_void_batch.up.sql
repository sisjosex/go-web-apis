-- INV-014 D3 — correcting and removing a lot, the debt INV-004 D1 left open.
--
-- Two operations, one guard. A lot that has been consumed has rows in
-- batch_movements, and each of those carries the cost_of_goods_sold already
-- booked against a sale. Correcting or voiding such a lot would rewrite money
-- that is already in the books, so both refuse with
-- inventory.batch.has-movements.
--
-- What is editable is deliberately narrow: lot_number and expiry_date, the two
-- fields a shop mistypes off the supplier's label. unit_cost is booked COGS and
-- current_quantity is owned by consumption; neither is editable under any
-- circumstances (INV-014 scope, "Out").
--
-- Removal is a status change, never a DELETE: inventory_movements and
-- sales.order_batch_assignments point at these rows, and a lot that was received
-- and then written off is history, not an error to erase.

-- ===========================================================================
-- sp_update_batch
-- ===========================================================================
DROP FUNCTION IF EXISTS inventory.sp_update_batch(UUID, UUID, VARCHAR, DATE);
CREATE FUNCTION inventory.sp_update_batch(
    p_tenant_id   UUID,
    p_batch_id    UUID,
    p_lot_number  VARCHAR DEFAULT NULL,
    p_expiry_date DATE    DEFAULT NULL
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
    days_to_expiry   INT,
    message          TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_status      VARCHAR;
    v_lot_number  VARCHAR;
    v_expiry_date DATE;
BEGIN
    SELECT pb.status, pb.lot_number, pb.expiry_date
    INTO v_status, v_lot_number, v_expiry_date
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

    -- The money guard: consumption has already priced this lot into sales.
    IF EXISTS (
        SELECT 1
        FROM inventory.batch_movements bm
        WHERE bm.batch_id = p_batch_id
    )
    THEN
        RAISE EXCEPTION 'inventory.batch.has-movements' USING ERRCODE = 'P0001';
    END IF;

    -- NULL means "leave it alone", so a caller can correct one field alone.
    IF p_lot_number IS NOT NULL
    THEN
        IF TRIM(p_lot_number) = ''
        THEN
            RAISE EXCEPTION 'batch.lot-number-required' USING ERRCODE = 'P0001';
        END IF;

        v_lot_number := TRIM(p_lot_number);
    END IF;

    IF p_expiry_date IS NOT NULL
    THEN
        IF p_expiry_date <= CURRENT_DATE
        THEN
            RAISE EXCEPTION 'batch.expiry-date-must-be-future'
                USING ERRCODE = 'P0001';
        END IF;

        v_expiry_date := p_expiry_date;
    END IF;

    -- The same expiry ladder sp_create_batch stamps at insert. Still stale by
    -- design (INV-004 D3) — recomputed here only because the date it derives
    -- from is exactly what just changed.
    IF v_expiry_date <= CURRENT_DATE
    THEN
        v_status := 'expired';
    ELSIF v_expiry_date <= CURRENT_DATE + INTERVAL '7 days'
    THEN
        v_status := 'expiring_soon';
    ELSE
        v_status := 'active';
    END IF;

    UPDATE inventory.product_batches pb
    SET lot_number  = v_lot_number,
        expiry_date = v_expiry_date,
        status      = v_status,
        updated_at  = CURRENT_TIMESTAMP
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
        CAST(pb.expiry_date - CURRENT_DATE AS INT),
        CAST('Batch updated successfully' AS TEXT)
    FROM inventory.product_batches pb
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.id = p_batch_id;
END;
$$;
COMMENT ON FUNCTION inventory.sp_update_batch IS
    'Corrects a lot''s lot_number and expiry_date, and nothing else (INV-014 '
    'D3). A NULL argument leaves that field as it is. Refuses with '
    'inventory.batch.has-movements once the lot has been consumed, because '
    'batch_movements already carries the COGS booked against it, and with '
    'inventory.batch.voided on a lot that was written off.';

-- ===========================================================================
-- sp_void_batch
-- ===========================================================================
DROP FUNCTION IF EXISTS inventory.sp_void_batch(UUID, UUID);
CREATE FUNCTION inventory.sp_void_batch(
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

    -- 'void' is the one authoritative value in this column: it is written by an
    -- explicit action, not stamped from a date. The FIFO pick below and the
    -- sales fulfilment SP both learn to skip it.
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
COMMENT ON FUNCTION inventory.sp_void_batch IS
    'Writes off a lot by setting status = ''void'' — never a DELETE, because '
    'inventory_movements and sales.order_batch_assignments point at the row and '
    'a lot that was received is history (INV-014 D3). Refuses a lot that has '
    'been consumed, and one that is already void.';

-- ===========================================================================
-- sp_get_oldest_batch_for_sale — 'void' leaves the FIFO pick
-- ===========================================================================
-- A written-off lot that still wins the pick is not written off. The status
-- filter here has excluded only 'expired' since 20260304000005, and INV-014
-- adds the fourth value the column can hold.
--
-- p_sku_id is untouched: 20260812000004 already added it, and this is a
-- body-only change, so CREATE OR REPLACE rather than DROP + CREATE.
--
-- sales.sp_add_order_item_with_batch carries the same filter and gets the same
-- change in its own module's migration; the two must agree about what is
-- sellable or the POS preview and the till disagree.
CREATE OR REPLACE FUNCTION inventory.sp_get_oldest_batch_for_sale(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_sku_id     UUID DEFAULT NULL
)
RETURNS TABLE(
    id               UUID,
    product_id       UUID,
    lot_number       VARCHAR,
    purchase_date    DATE,
    expiry_date      DATE,
    unit_cost        DECIMAL,
    current_quantity DECIMAL,
    status           VARCHAR,
    message          TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_sku_id UUID;
BEGIN
    -- Resolved leniently rather than through fn_resolve_sku: this SP answered an
    -- unknown product with zero rows (which the caller turns into a 404) and it
    -- has to keep doing that instead of raising.
    SELECT s.id INTO v_sku_id
    FROM inventory.product_skus s
    WHERE s.product_id = p_product_id
      AND s.tenant_id = p_tenant_id
      AND ((p_sku_id IS NULL AND s.is_default) OR s.id = p_sku_id);

    IF v_sku_id IS NULL
    THEN
        RETURN;
    END IF;

    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.lot_number,
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.current_quantity,
        pb.status,
        'Batch retrieved for sale'::TEXT
    FROM inventory.product_batches pb
    WHERE pb.tenant_id = p_tenant_id
        AND pb.sku_id = v_sku_id
        AND pb.status NOT IN ('expired', 'void')
        AND pb.current_quantity > 0
    ORDER BY pb.expiry_date ASC, pb.created_at ASC
    LIMIT 1;
END;
$$;
COMMENT ON FUNCTION inventory.sp_get_oldest_batch_for_sale IS
    'FIFO pick for a SKU (the product default when p_sku_id is omitted): the '
    'oldest lot with quantity left that is neither expired nor voided '
    '(INV-014 D3). Phase 3 owns picking across several SKUs of one product.';
