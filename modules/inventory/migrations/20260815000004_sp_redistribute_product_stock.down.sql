-- Reverses 20260815000004. The movements and quantities it wrote stay: a
-- redistribution is a stock event, not a schema one.
DROP FUNCTION IF EXISTS inventory.sp_redistribute_product_stock(
    UUID, UUID, JSONB, UUID
);
