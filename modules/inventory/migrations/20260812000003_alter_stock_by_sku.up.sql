-- INV-007 phase 1 — move every quantity off the product and onto a SKU.
--
-- This runs once per tenant database (make tenant-migrate SLUG=x) and a
-- half-applied tenant must be re-runnable, so every statement is guarded.
--
-- The backfill drives off products, not off product_stock: nothing guarantees
-- a product went through sp_create_product_with_variants, so a product with no
-- stock row exists and gets one at 0 before any NOT NULL is set.

-- ---------------------------------------------------------------------------
-- 1. One default SKU per product — sku copied verbatim (INV-007 D2b)
-- ---------------------------------------------------------------------------
INSERT INTO inventory.product_skus (
    tenant_id, product_id, sku, price_modifier, combination_key, is_default, status
)
SELECT p.tenant_id, p.id, p.sku, 0, '', TRUE, 'active'
FROM inventory.products p
WHERE NOT EXISTS (
    SELECT 1
    FROM inventory.product_skus s
    WHERE s.product_id = p.id
      AND s.is_default
)
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. The stock rows nobody created
-- ---------------------------------------------------------------------------
INSERT INTO inventory.product_stock (
    product_id, current_quantity, reserved_quantity, reorder_level, status
)
SELECT p.id, 0, 0, 10, inventory.fn_stock_status(0, 10)
FROM inventory.products p
WHERE NOT EXISTS (
    SELECT 1
    FROM inventory.product_stock ps
    WHERE ps.product_id = p.id
);

-- ---------------------------------------------------------------------------
-- 3. sku_id on the three quantity-bearing tables, backfilled to the default
-- ---------------------------------------------------------------------------
ALTER TABLE inventory.product_stock       ADD COLUMN IF NOT EXISTS sku_id UUID;
ALTER TABLE inventory.inventory_movements ADD COLUMN IF NOT EXISTS sku_id UUID;
ALTER TABLE inventory.product_batches     ADD COLUMN IF NOT EXISTS sku_id UUID;

UPDATE inventory.product_stock ps
SET sku_id = s.id
FROM inventory.product_skus s
WHERE s.product_id = ps.product_id
  AND s.is_default
  AND ps.sku_id IS NULL;

UPDATE inventory.inventory_movements im
SET sku_id = s.id
FROM inventory.product_skus s
WHERE s.product_id = im.product_id
  AND s.is_default
  AND im.sku_id IS NULL;

UPDATE inventory.product_batches pb
SET sku_id = s.id
FROM inventory.product_skus s
WHERE s.product_id = pb.product_id
  AND s.is_default
  AND pb.sku_id IS NULL;

ALTER TABLE inventory.product_stock       ALTER COLUMN sku_id SET NOT NULL;
ALTER TABLE inventory.inventory_movements ALTER COLUMN sku_id SET NOT NULL;
ALTER TABLE inventory.product_batches     ALTER COLUMN sku_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_product_stock_sku_id
    ON inventory.product_stock(sku_id);
CREATE INDEX IF NOT EXISTS idx_movements_sku_id
    ON inventory.inventory_movements(sku_id);
CREATE INDEX IF NOT EXISTS idx_product_batches_sku_id
    ON inventory.product_batches(sku_id);

-- ---------------------------------------------------------------------------
-- 4. Composite FKs — product_id stays for cheap per-product queries, and
--    Postgres guarantees it can never disagree with the SKU
-- ---------------------------------------------------------------------------
ALTER TABLE inventory.product_stock
    DROP CONSTRAINT IF EXISTS product_stock_sku_product_fkey;
ALTER TABLE inventory.product_stock
    ADD CONSTRAINT product_stock_sku_product_fkey
    FOREIGN KEY (sku_id, product_id)
    REFERENCES inventory.product_skus(id, product_id) ON DELETE CASCADE;

ALTER TABLE inventory.inventory_movements
    DROP CONSTRAINT IF EXISTS inventory_movements_sku_product_fkey;
ALTER TABLE inventory.inventory_movements
    ADD CONSTRAINT inventory_movements_sku_product_fkey
    FOREIGN KEY (sku_id, product_id)
    REFERENCES inventory.product_skus(id, product_id);

ALTER TABLE inventory.product_batches
    DROP CONSTRAINT IF EXISTS product_batches_sku_product_fkey;
ALTER TABLE inventory.product_batches
    ADD CONSTRAINT product_batches_sku_product_fkey
    FOREIGN KEY (sku_id, product_id)
    REFERENCES inventory.product_skus(id, product_id) ON DELETE CASCADE;

-- ---------------------------------------------------------------------------
-- 5. Stock is unique per SKU, no longer per product
-- ---------------------------------------------------------------------------
ALTER TABLE inventory.product_stock
    DROP CONSTRAINT IF EXISTS product_stock_product_id_key;
ALTER TABLE inventory.product_stock
    DROP CONSTRAINT IF EXISTS product_stock_sku_id_key;
ALTER TABLE inventory.product_stock
    ADD CONSTRAINT product_stock_sku_id_key UNIQUE (sku_id);

-- ---------------------------------------------------------------------------
-- 6. Per-product totals, so sp_get_product_stock keeps returning one row
-- ---------------------------------------------------------------------------
CREATE OR REPLACE VIEW inventory.v_product_stock_totals AS
SELECT
    ps.product_id,
    SUM(ps.current_quantity)                        AS current_quantity,
    SUM(ps.reserved_quantity)                       AS reserved_quantity,
    SUM(ps.current_quantity - ps.reserved_quantity) AS available_quantity,
    SUM(ps.reorder_level)                           AS reorder_level,
    inventory.fn_stock_status(
        SUM(ps.current_quantity),
        SUM(ps.reorder_level)
    )                                               AS status,
    MAX(ps.last_updated_at)                         AS last_updated_at
FROM inventory.product_stock ps
GROUP BY ps.product_id;

COMMENT ON VIEW inventory.v_product_stock_totals IS
    'Per-product sums over the product SKUs. status is recomputed through '
    'inventory.fn_stock_status so the product-level answer can never contradict '
    'the per-SKU rows it sums.';

COMMENT ON COLUMN inventory.product_stock.sku_id IS
    'The SKU these quantities belong to. product_id is kept alongside it and '
    'the composite FK guarantees the two agree.';
COMMENT ON COLUMN inventory.inventory_movements.sku_id IS
    'The SKU this movement applied to; the product default until phase 2 '
    'generates real combinations.';
COMMENT ON COLUMN inventory.product_batches.sku_id IS
    'The SKU this batch belongs to; the product default until phase 2 '
    'generates real combinations.';
