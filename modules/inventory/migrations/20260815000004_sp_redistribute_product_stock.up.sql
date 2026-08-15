-- INV-008 phase 2 — D4's redistribution: the whole unassigned bucket moves onto
-- real combinations in one transaction, or nothing moves.
--
-- Three properties the contract rests on:
--   * the amounts must sum to EXACTLY the default SKU's current quantity. A
--     partial move would leave a product half-migrated indefinitely, and a move
--     that does not add up is a client bug, not a rounding detail
--   * every row it touches is locked in ascending sku_id order. INV-007 added
--     FOR UPDATE to three SPs and noted that a caller touching several SKUs must
--     take them in a stable order; this is the first caller that does, and
--     phase 3 inherits the rule
--   * the ledger balances. One positive TRANSFER per target (D4) and one
--     negative TRANSFER on the bucket they came out of, so summing
--     inventory_movements per SKU keeps reproducing product_stock

DROP FUNCTION IF EXISTS inventory.sp_redistribute_product_stock(
    UUID, UUID, JSONB, UUID
);
CREATE FUNCTION inventory.sp_redistribute_product_stock(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_targets    JSONB,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE(
    product_id     UUID,
    default_sku_id UUID,
    moved_quantity DECIMAL,
    target_count   INT,
    message        TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_default_sku_id UUID;
    v_target_ids     UUID[];
    v_target_count   INT;
    v_total          DECIMAL;
    v_current        DECIMAL;
    v_reserved       DECIMAL;
BEGIN
    -- Validates the product against the tenant and finds the bucket (INV-007 D4)
    v_default_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, NULL);

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

    -- A repeated sku_id would make the amounts ambiguous: the last write would
    -- win and the sum would still look right.
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

    -- Every target is a generated combination of this product. The bucket is
    -- the source, never a destination.
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

    -- Lock the bucket and every target together, in a stable order, before
    -- reading the quantity the sum is checked against.
    PERFORM ps.sku_id
    FROM inventory.product_stock ps
    WHERE ps.sku_id = ANY (v_target_ids || v_default_sku_id)
    ORDER BY ps.sku_id
    FOR UPDATE;

    SELECT ps.current_quantity, ps.reserved_quantity
    INTO v_current, v_reserved
    FROM inventory.product_stock ps
    WHERE ps.sku_id = v_default_sku_id;

    -- Emptying a bucket that still backs a reservation would leave the product
    -- with negative availability on a SKU nobody can sell from.
    IF v_reserved > 0
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-reserved'
            USING ERRCODE = 'P0001';
    END IF;

    IF v_total <> v_current
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-mismatch'
            USING ERRCODE = 'P0001';
    END IF;

    UPDATE inventory.product_stock ps
    SET current_quantity = ps.current_quantity + t.quantity,
        status           = inventory.fn_stock_status(
            ps.current_quantity + t.quantity, ps.reorder_level
        ),
        last_updated_at  = CURRENT_TIMESTAMP
    FROM (
        SELECT
            (e ->> 'sku_id')::UUID     AS sku_id,
            (e ->> 'quantity')::DECIMAL AS quantity
        FROM jsonb_array_elements(p_targets) e
    ) t
    WHERE ps.sku_id = t.sku_id;

    -- D4 — one TRANSFER per target, so the audit trail says where the units went
    INSERT INTO inventory.inventory_movements (
        tenant_id, product_id, sku_id, movement_type, quantity,
        reference_type, reference_id, notes, created_by
    )
    SELECT
        p_tenant_id,
        p_product_id,
        (e ->> 'sku_id')::UUID,
        'TRANSFER',
        (e ->> 'quantity')::DECIMAL,
        'REDISTRIBUTION',
        v_default_sku_id,
        'Redistributed from the product default SKU',
        p_created_by
    FROM jsonb_array_elements(p_targets) e;

    UPDATE inventory.product_stock ps
    SET current_quantity = 0,
        status           = inventory.fn_stock_status(0, ps.reorder_level),
        last_updated_at  = CURRENT_TIMESTAMP
    WHERE ps.sku_id = v_default_sku_id;

    -- The balancing entry. Without it the ledger says the units were created.
    INSERT INTO inventory.inventory_movements (
        tenant_id, product_id, sku_id, movement_type, quantity,
        reference_type, reference_id, notes, created_by
    )
    VALUES (
        p_tenant_id, p_product_id, v_default_sku_id, 'TRANSFER', -v_total,
        'REDISTRIBUTION', v_default_sku_id,
        'Redistributed to the product combinations', p_created_by
    );

    RETURN QUERY SELECT
        p_product_id,
        v_default_sku_id,
        CAST(v_total AS DECIMAL),
        CAST(v_target_count AS INT),
        CAST('Stock redistributed' AS TEXT);
END;
$$;
COMMENT ON FUNCTION inventory.sp_redistribute_product_stock IS
    'Moves the whole default-SKU bucket onto generated combinations in one '
    'transaction (INV-008 D4). The amounts must sum to exactly the bucket''s '
    'current quantity; anything else raises and changes nothing. Locks the '
    'bucket and every target in ascending sku_id order, and records one TRANSFER '
    'per target plus the balancing entry on the bucket.';
