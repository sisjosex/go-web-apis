-- INV-020 D1: who created and last changed each row, written by the SP that already makes the change,
-- so no extra write per change. No foreign key: auth may live in another database.
ALTER TABLE inventory.products
    ADD COLUMN created_by UUID,
    ADD COLUMN updated_by UUID;

ALTER TABLE inventory.product_categories
    ADD COLUMN created_by UUID,
    ADD COLUMN updated_by UUID;

ALTER TABLE inventory.product_batches
    ADD COLUMN created_by UUID,
    ADD COLUMN updated_by UUID;

ALTER TABLE inventory.product_stock
    ADD COLUMN updated_by UUID;
