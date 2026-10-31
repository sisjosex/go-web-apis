ALTER TABLE inventory.product_stock DROP COLUMN IF EXISTS updated_by;
ALTER TABLE inventory.product_batches DROP COLUMN IF EXISTS updated_by, DROP COLUMN IF EXISTS created_by;
ALTER TABLE inventory.product_categories DROP COLUMN IF EXISTS updated_by, DROP COLUMN IF EXISTS created_by;
ALTER TABLE inventory.products DROP COLUMN IF EXISTS updated_by, DROP COLUMN IF EXISTS created_by;
