-- INV-023: as on a sale line: the presentation and quantity as bought, stock from base_quantity.
ALTER TABLE purchasing.purchase_order_items DROP COLUMN line_total;
ALTER TABLE purchasing.purchase_order_items ALTER COLUMN quantity TYPE NUMERIC(14,3);
ALTER TABLE purchasing.purchase_order_items ALTER COLUMN received_quantity TYPE NUMERIC(14,3);
ALTER TABLE purchasing.purchase_order_items
    ADD COLUMN line_total NUMERIC(14,2) GENERATED ALWAYS AS (quantity * unit_cost) STORED,
    ADD COLUMN product_unit_id UUID REFERENCES inventory.product_units (id),
    ADD COLUMN unit_factor NUMERIC(14,3) NOT NULL DEFAULT 1,
    ADD COLUMN base_quantity NUMERIC(14,3) GENERATED ALWAYS AS (quantity * unit_factor) STORED;
