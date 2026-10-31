ALTER TABLE inventory.product_batches DROP CONSTRAINT IF EXISTS chk_product_batches_quantity;
ALTER TABLE inventory.product_stock DROP CONSTRAINT IF EXISTS chk_product_stock_quantities;
