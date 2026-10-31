-- INV-027: a lot belongs to a warehouse, and what a transfer took from each lot is kept to receive it the same.
INSERT INTO inventory.warehouses (tenant_id, name, is_default)
SELECT DISTINCT b.tenant_id, 'Principal', TRUE
FROM inventory.product_batches b
WHERE NOT EXISTS (SELECT 1 FROM inventory.warehouses w WHERE w.tenant_id = b.tenant_id AND w.is_default);

ALTER TABLE inventory.product_batches ADD COLUMN warehouse_id UUID REFERENCES inventory.warehouses (id);
UPDATE inventory.product_batches b
SET warehouse_id = w.id
FROM inventory.warehouses w
WHERE w.tenant_id = b.tenant_id AND w.is_default;
ALTER TABLE inventory.product_batches ALTER COLUMN warehouse_id SET NOT NULL;
CREATE INDEX idx_product_batches_warehouse_sku_expiry ON inventory.product_batches (warehouse_id, sku_id, expiry_date);

CREATE TABLE inventory.transfer_line_batches (
    transfer_line_id UUID          NOT NULL REFERENCES inventory.transfer_lines (id) ON DELETE CASCADE,
    source_batch_id  UUID          NOT NULL REFERENCES inventory.product_batches (id),
    quantity         NUMERIC(14,3) NOT NULL CHECK (quantity > 0),
    position         INT           NOT NULL,
    PRIMARY KEY (transfer_line_id, source_batch_id)
);
