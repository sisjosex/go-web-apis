-- INV-018: what the SPs promise, kept by the database. Runs on the data INV-028 emptied; a row that
-- breaks one of these fails the migration, it is never silently corrected.
ALTER TABLE inventory.product_stock
    ADD CONSTRAINT chk_product_stock_quantities CHECK (current_quantity >= 0 AND reserved_quantity >= 0 AND reserved_quantity <= current_quantity);

ALTER TABLE inventory.product_batches
    ADD CONSTRAINT chk_product_batches_quantity CHECK (current_quantity >= 0);
