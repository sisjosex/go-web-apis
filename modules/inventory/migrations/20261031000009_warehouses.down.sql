DROP INDEX IF EXISTS inventory.idx_product_stock_warehouse_sku;
ALTER TABLE inventory.product_stock DROP CONSTRAINT IF EXISTS product_stock_sku_warehouse_key;
ALTER TABLE inventory.product_stock ADD CONSTRAINT product_stock_sku_id_key UNIQUE (sku_id);
ALTER TABLE inventory.inventory_movements DROP COLUMN IF EXISTS warehouse_id;
ALTER TABLE inventory.product_stock DROP COLUMN IF EXISTS warehouse_id;
DROP TABLE IF EXISTS inventory.warehouses;
