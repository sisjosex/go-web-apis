-- Reverses 20260816000001. Drops in the opposite order to the up: the two D3
-- composite FKs first, then the assignments column, then the order-items column
-- and the helper unique indexes the FKs required.
--
-- What is left is a database the pre-INV-010 binary serves correctly: both
-- tables are byte-for-byte their 20260305000006 / 20260303000003 shape, and
-- inventory.product_batches keeps its own sku_id, which belongs to INV-007 and
-- is not this migration's to remove.

ALTER TABLE sales.order_batch_assignments
    DROP CONSTRAINT IF EXISTS order_batch_assignments_batch_sku_fkey;
ALTER TABLE sales.order_batch_assignments
    DROP CONSTRAINT IF EXISTS order_batch_assignments_item_sku_fkey;

DROP INDEX IF EXISTS sales.idx_order_batch_assignments_sku_id;

ALTER TABLE sales.order_batch_assignments
    DROP COLUMN IF EXISTS sku_id;

ALTER TABLE sales.order_items
    DROP CONSTRAINT IF EXISTS order_items_sku_product_fkey;

DROP INDEX IF EXISTS sales.idx_order_items_sku_id;

ALTER TABLE sales.order_items
    DROP COLUMN IF EXISTS sku_id;

-- Dropped last: idx_order_items_id_sku_id disappears with the column above on
-- most versions, but naming it keeps the down explicit and re-runnable.
DROP INDEX IF EXISTS sales.idx_order_items_id_sku_id;
DROP INDEX IF EXISTS inventory.idx_product_batches_id_sku_id;
