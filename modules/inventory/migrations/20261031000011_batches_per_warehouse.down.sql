DROP TABLE IF EXISTS inventory.transfer_line_batches;
DROP INDEX IF EXISTS inventory.idx_product_batches_warehouse_sku_expiry;
ALTER TABLE inventory.product_batches DROP COLUMN IF EXISTS warehouse_id;
