-- Drop indexes
DROP INDEX IF EXISTS inventory.idx_inventory_movements_batch_id;
DROP INDEX IF EXISTS inventory.idx_batch_movements_movement_id;
DROP INDEX IF EXISTS inventory.idx_batch_movements_batch_id;
DROP INDEX IF EXISTS inventory.idx_product_batches_product_expiry;
DROP INDEX IF EXISTS inventory.idx_product_batches_expiry_date;
DROP INDEX IF EXISTS inventory.idx_product_batches_product_id;

-- Drop tables
DROP TABLE IF EXISTS inventory.batch_movements;
DROP TABLE IF EXISTS inventory.product_batches;

-- Remove batch_id column from inventory_movements
ALTER TABLE inventory.inventory_movements 
DROP COLUMN IF EXISTS batch_id;
