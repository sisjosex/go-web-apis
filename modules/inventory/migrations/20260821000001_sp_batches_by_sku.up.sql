-- INV-014 — a lot belongs to a combination, so every batch read has to say
-- which one.
--
-- sp_list_batches_by_product filtered on product_id alone, so the Batches tab
-- mixed every combination into one flat list that never said which was which.
-- Both SPs gain sku_id and sku, and the per-product list gains an optional
-- p_sku_id filter appended LAST, so existing positional callers keep meaning
-- what they mean today.
--
-- sp_get_oldest_batch_for_sale already takes p_sku_id (20260812000004) and is
-- deliberately NOT redefined here — only Go stops omitting the argument.
--
-- Return types change, so DROP + CREATE rather than CREATE OR REPLACE.

-- ===========================================================================
-- sp_list_batches_by_product
-- ===========================================================================
DROP FUNCTION IF EXISTS inventory.sp_list_batches_by_product(UUID, UUID, BOOLEAN);
CREATE FUNCTION inventory.sp_list_batches_by_product(
    p_tenant_id   UUID,
    p_product_id  UUID,
    p_only_active BOOLEAN DEFAULT TRUE,
    p_sku_id      UUID DEFAULT NULL
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
    days_to_expiry   INT,
    sku_id           UUID,
    sku              VARCHAR
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(pb.status AS VARCHAR),
        CAST(pb.expiry_date - CURRENT_DATE AS INT),
        pb.sku_id,
        CAST(s.sku AS VARCHAR)
    FROM inventory.product_batches pb
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.tenant_id = p_tenant_id
        AND pb.product_id = p_product_id
        AND (p_sku_id IS NULL OR pb.sku_id = p_sku_id)
        AND (NOT p_only_active OR pb.status NOT IN ('expired', 'void'))
    ORDER BY pb.expiry_date ASC, pb.created_at ASC;
END;
$$;
COMMENT ON FUNCTION inventory.sp_list_batches_by_product IS
    'Lots of a product in FIFO order, each carrying the combination it belongs '
    'to. p_sku_id narrows the list to one combination; omitted, it returns every '
    'combination. p_only_active hides expired and voided lots (INV-014).';

-- ===========================================================================
-- sp_get_expiring_batches
-- ===========================================================================
DROP FUNCTION IF EXISTS inventory.sp_get_expiring_batches(UUID, INT);
CREATE FUNCTION inventory.sp_get_expiring_batches(
    p_tenant_id    UUID,
    p_warning_days INT
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
    days_to_expiry   INT,
    product_name     VARCHAR,
    sku_id           UUID,
    sku              VARCHAR
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(pb.status AS VARCHAR),
        CAST(pb.expiry_date - CURRENT_DATE AS INT),
        CAST(p.name AS VARCHAR),
        pb.sku_id,
        CAST(s.sku AS VARCHAR)
    FROM inventory.product_batches pb
    INNER JOIN inventory.products p
        ON p.id = pb.product_id
        AND p.tenant_id = pb.tenant_id
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.tenant_id = p_tenant_id
        AND pb.expiry_date - CURRENT_DATE <= p_warning_days
        AND pb.expiry_date - CURRENT_DATE > 0
        AND pb.status IN ('active', 'expiring_soon')
    ORDER BY pb.expiry_date ASC;
END;
$$;
COMMENT ON FUNCTION inventory.sp_get_expiring_batches IS
    'Lots expiring within the warning window, with the product name and the '
    'combination each lot belongs to (INV-014). A voided lot never appears: the '
    'status filter already excludes anything that is not active or expiring.';
