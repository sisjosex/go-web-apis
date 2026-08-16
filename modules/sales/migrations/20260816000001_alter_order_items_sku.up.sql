-- INV-010 phase 3 (sales half) — an order line names the combination it sold.
--
-- INV-007 put the quantities on SKUs and INV-008 generated the combinations;
-- sales still carried product_id alone, so an order for the Polera in M was
-- happily fulfilled out of the XL lot. This migration adds the column on both
-- sales tables and, following INV-007 and INV-008, states the agreement between
-- an item and the batches that fulfil it as composite FKs rather than as a
-- filter living inside one SP (INV-010 D3).
--
-- Runs once per tenant database (make tenant-migrate SLUG=x) and a half-applied
-- tenant has to survive being migrated again, so every statement is guarded.

-- ---------------------------------------------------------------------------
-- 1. sales.order_items.sku_id — backfilled to each product's default SKU
-- ---------------------------------------------------------------------------
-- Every pre-INV-010 row was sold before combinations existed, so it belongs to
-- the default SKU: the "unassigned" bucket INV-007 D4 created and 20260812000003
-- guaranteed for every product.
ALTER TABLE sales.order_items
    ADD COLUMN IF NOT EXISTS sku_id UUID;

UPDATE sales.order_items oi
SET sku_id = s.id
FROM inventory.product_skus s
WHERE s.product_id = oi.product_id
  AND s.is_default
  AND oi.sku_id IS NULL;

ALTER TABLE sales.order_items
    ALTER COLUMN sku_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_order_items_sku_id
    ON sales.order_items(sku_id);

-- product_id stays for cheap per-product reporting, and the composite FK
-- guarantees it can never disagree with the SKU. No ON DELETE CASCADE: an order
-- line is a historical record and must outlive the SKU catalogue, exactly as
-- inventory.inventory_movements is kept in 20260812000003.
ALTER TABLE sales.order_items
    DROP CONSTRAINT IF EXISTS order_items_sku_product_fkey;
ALTER TABLE sales.order_items
    ADD CONSTRAINT order_items_sku_product_fkey
    FOREIGN KEY (sku_id, product_id)
    REFERENCES inventory.product_skus(id, product_id);

COMMENT ON COLUMN sales.order_items.sku_id IS
    'The sellable combination this line sold. Backfilled to the product default '
    'for every row that predates INV-010; from the first generated combination '
    'on it is the SKU the client named. product_id is carried alongside it and '
    'order_items_sku_product_fkey guarantees the two agree.';

-- ---------------------------------------------------------------------------
-- 2. Helper unique indexes the D3 composite FKs require
-- ---------------------------------------------------------------------------
-- A composite FK needs a unique index on exactly its referenced column pair.
-- Both are redundant with an existing primary key on the first column alone,
-- which is precisely what makes them safe: they cannot reject a row the PK
-- accepts, they only give Postgres something to point the FK at. Same device as
-- idx_product_skus_id_product_id in 20260812000001.
CREATE UNIQUE INDEX IF NOT EXISTS idx_order_items_id_sku_id
    ON sales.order_items(id, sku_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_batches_id_sku_id
    ON inventory.product_batches(id, sku_id);

-- ---------------------------------------------------------------------------
-- 3. sales.order_batch_assignments.sku_id — backfilled from its own batch
-- ---------------------------------------------------------------------------
-- Driven off the batch and not off the order item on purpose: the batch is where
-- the SKU physically is, and 20260812000003 already set every batch to its
-- product default. For existing rows the two agree by construction — the FIFO
-- loop only ever assigned batches of the item's own product — so the composite
-- FKs added below hold on the backfilled data.
ALTER TABLE sales.order_batch_assignments
    ADD COLUMN IF NOT EXISTS sku_id UUID;

UPDATE sales.order_batch_assignments oba
SET sku_id = pb.sku_id
FROM inventory.product_batches pb
WHERE pb.id = oba.product_batch_id
  AND oba.sku_id IS NULL;

ALTER TABLE sales.order_batch_assignments
    ALTER COLUMN sku_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_order_batch_assignments_sku_id
    ON sales.order_batch_assignments(sku_id);

-- ---------------------------------------------------------------------------
-- 4. INV-010 D3 — fulfilling an order for M out of the XL batch is rejected by
--    Postgres, not merely avoided by the picking loop
-- ---------------------------------------------------------------------------
-- The assignment's SKU must be the order item's SKU...
ALTER TABLE sales.order_batch_assignments
    DROP CONSTRAINT IF EXISTS order_batch_assignments_item_sku_fkey;
ALTER TABLE sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_item_sku_fkey
    FOREIGN KEY (order_item_id, sku_id)
    REFERENCES sales.order_items(id, sku_id) ON DELETE CASCADE;

-- ...and the batch it draws from must hold that same SKU. Together they close
-- the invariant to every writer, including the next hand-written INSERT that
-- will not have read sp_add_order_item_with_batch.
ALTER TABLE sales.order_batch_assignments
    DROP CONSTRAINT IF EXISTS order_batch_assignments_batch_sku_fkey;
ALTER TABLE sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_batch_sku_fkey
    FOREIGN KEY (product_batch_id, sku_id)
    REFERENCES inventory.product_batches(id, sku_id);

COMMENT ON COLUMN sales.order_batch_assignments.sku_id IS
    'The combination this assignment moves. Duplicated from both sides on '
    'purpose: the two composite FKs make it impossible for an assignment to '
    'name an order item of one SKU and a batch of another (INV-010 D3).';
