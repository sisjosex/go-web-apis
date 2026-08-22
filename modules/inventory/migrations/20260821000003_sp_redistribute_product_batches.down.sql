-- INV-014 D2 introduced this; there is no earlier definition to restore.
--
-- The lots it split are NOT re-merged. Each child is a real row that stock,
-- movements and possibly sales already reference, and folding them back into
-- the emptied parent would move units the ledger has already accounted for.
DROP FUNCTION IF EXISTS inventory.sp_redistribute_product_batches(
    UUID, UUID, UUID, JSONB, UUID
);
