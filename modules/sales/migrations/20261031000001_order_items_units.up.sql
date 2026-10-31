-- INV-023: a line keeps its presentation and quantity as sold; stock reads base_quantity. line_total depends on
-- quantity, so it goes and comes back around the type change (the table is empty after INV-028).
ALTER TABLE sales.order_items DROP COLUMN line_total;
ALTER TABLE sales.order_items ALTER COLUMN quantity TYPE NUMERIC(14,3);
ALTER TABLE sales.order_items
    ADD COLUMN line_total NUMERIC(14,2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
    ADD COLUMN product_unit_id UUID REFERENCES inventory.product_units (id),
    ADD COLUMN unit_factor NUMERIC(14,3) NOT NULL DEFAULT 1,
    ADD COLUMN base_quantity NUMERIC(14,3) GENERATED ALWAYS AS (quantity * unit_factor) STORED;
ALTER TABLE sales.order_batch_assignments ALTER COLUMN quantity_assigned TYPE NUMERIC(14,3);
