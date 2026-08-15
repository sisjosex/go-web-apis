-- Reverses 20260815000001: the combination_key trigger and stock_by_variant.
DROP TRIGGER IF EXISTS trg_sku_options_combination_key
    ON inventory.product_sku_options;

DROP FUNCTION IF EXISTS inventory.fn_check_sku_combination_key();

ALTER TABLE inventory.products
    DROP COLUMN IF EXISTS stock_by_variant;
