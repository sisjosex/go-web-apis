ALTER TABLE sales.order_batch_assignments ALTER COLUMN quantity_assigned TYPE NUMERIC(10,2);
ALTER TABLE sales.order_items DROP COLUMN base_quantity, DROP COLUMN unit_factor, DROP COLUMN product_unit_id, DROP COLUMN line_total;
ALTER TABLE sales.order_items ALTER COLUMN quantity TYPE INTEGER;
ALTER TABLE sales.order_items ADD COLUMN line_total NUMERIC(12,2) GENERATED ALWAYS AS (quantity::NUMERIC * unit_price) STORED;
