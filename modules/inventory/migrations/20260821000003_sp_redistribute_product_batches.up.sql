-- INV-014 D2 — moving the lots stranded in the unassigned bucket onto real
-- combinations.
--
-- product_batches.sku_id has been NOT NULL since INV-007 and every lot the API
-- ever created resolved to the product's default SKU, so on a stock_by_variant
-- product this is never an empty set: those lots exist, hold quantity, and
-- sp_add_order_item_with_batch can never reach them.
--
-- It SPLITS, it does not reassign. An UPDATE of sku_id is one statement and it
-- is wrong: batch_movements rows point at the lot and carry the COGS already
-- booked against sales, so moving a consumed lot rewrites money in closed
-- books. Instead the original shrinks and one child per target is inserted,
-- carrying the same lot_number, purchase_date, expiry_date and unit_cost. The
-- history stays attached to the original and nothing is rewritten.
--
-- A lot that has ANY batch_movements row is refused outright rather than
-- partially split: its remaining quantity cannot be told apart from the part
-- already sold without re-deriving per-combination COGS, which is exactly the
-- rewrite this avoids.
--
-- It touches product_batches and NOTHING ELSE. Lots and stock are two separate
-- ledgers in this schema and always have been: sp_create_batch never adds to
-- product_stock, and sales.sp_complete_sales_order consumes a lot without
-- subtracting from it either. Only sp_record_movement moves stock. An earlier
-- draft of this SP copied sp_redistribute_product_stock's ledger entries and so
-- moved quantities that had never entered stock — on the real data, where a
-- product carries lots and a stock row of 0, it drove the bucket to -10.
--
-- Shaped on sp_redistribute_product_stock (20260815000004) for its argument
-- validation and its one contract rule that does apply here: the amounts must
-- sum to EXACTLY the lot's current quantity, or nothing moves.

DROP FUNCTION IF EXISTS inventory.sp_redistribute_product_batches(
    UUID, UUID, UUID, JSONB, UUID
);
CREATE FUNCTION inventory.sp_redistribute_product_batches(
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
    -- Validates the product against the tenant and finds the bucket (INV-007 D4)
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

    -- Only the bucket is a source. A lot already sitting on a combination is
    -- where it belongs, and moving it would be a transfer, not a migration.
    IF v_batch.sku_id <> v_default_sku_id
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-mismatch'
            USING ERRCODE = 'P0001';
    END IF;

    IF v_batch.status = 'void'
    THEN
        RAISE EXCEPTION 'inventory.batch.voided' USING ERRCODE = 'P0001';
    END IF;

    -- The money guard, same one sp_update_batch and sp_void_batch apply.
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

    -- A repeated sku_id would make the amounts ambiguous: two children on one
    -- combination, and the sum would still look right.
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

    -- A partial split would leave the lot half-migrated indefinitely, and one
    -- that does not add up is a client bug, not a rounding detail.
    IF v_total <> v_batch.current_quantity
    THEN
        RAISE EXCEPTION 'inventory.skus.redistribution-mismatch'
            USING ERRCODE = 'P0001';
    END IF;

    -- One child per target. initial_quantity is the child's own amount, not the
    -- parent's: the child is the lot as it exists on that combination, and
    -- INV-004's "consumed" reading of initial - current has to stay true.
    -- The ids are captured here rather than re-found afterwards: parent and
    -- children share a lot_number by design, so no WHERE clause can tell them
    -- apart reliably.
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

    -- The parent is emptied, never deleted: sales.order_batch_assignments and
    -- inventory_movements may point at it, and it is the row the history hangs
    -- off. 'depleted' is the existing word for a lot with nothing left.
    UPDATE inventory.product_batches pb
    SET current_quantity = 0,
        status           = 'depleted',
        updated_at       = CURRENT_TIMESTAMP
    WHERE pb.id = p_batch_id;

    -- No stock row is written and no inventory_movement is recorded. The units
    -- were never in product_stock to begin with, so "moving" them there would
    -- invent stock on the targets and drive the bucket negative; and an
    -- inventory_movement that does not move stock would break the invariant that
    -- summing movements per SKU reproduces product_stock (INV-008). The split is
    -- recorded where it happened: the emptied parent and its children, which
    -- share a lot number.
    --
    -- p_created_by is therefore unused today. It stays in the signature because
    -- the moment lots and stock are reconciled — the gap this SP deliberately
    -- does not close — this is the operation that will need to say who did it.

    -- The children, in the order the combinations sort, so the caller can show
    -- what it produced.
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
    'Splits one default-SKU lot onto generated combinations in a single '
    'transaction (INV-014 D2). The amounts must sum to exactly the lot''s '
    'current quantity; anything else raises and changes nothing. The original '
    'is emptied and kept, the children carry its lot number, dates and unit '
    'cost, and a lot with any batch_movements row is refused rather than '
    'split — an UPDATE of sku_id would rewrite COGS already booked. Touches '
    'product_batches only: lots and product_stock are separate ledgers here, so '
    'writing stock would invent units on the targets and drive the bucket '
    'negative.';
