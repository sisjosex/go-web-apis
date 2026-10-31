ALTER TABLE purchasing.purchase_order_items DROP COLUMN base_quantity, DROP COLUMN unit_factor, DROP COLUMN product_unit_id, DROP COLUMN line_total;
ALTER TABLE purchasing.purchase_order_items ALTER COLUMN received_quantity TYPE INTEGER;
ALTER TABLE purchasing.purchase_order_items ALTER COLUMN quantity TYPE INTEGER;
ALTER TABLE purchasing.purchase_order_items ADD COLUMN line_total NUMERIC(12,2) GENERATED ALWAYS AS (quantity::NUMERIC * unit_cost) STORED;
