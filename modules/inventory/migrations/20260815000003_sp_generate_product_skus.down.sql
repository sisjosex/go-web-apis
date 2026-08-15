-- Reverses 20260815000003. Nothing preceded this SP, so the down is a drop;
-- the rows it created stay, since deleting SKUs would delete stock with them.
DROP FUNCTION IF EXISTS inventory.sp_generate_product_skus(UUID, UUID, INT, INT);
