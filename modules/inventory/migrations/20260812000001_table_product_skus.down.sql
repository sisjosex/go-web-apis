-- Reverse of 20260812000001 — leaves the pre-INV-007 catalogue exactly as it was.

DROP TRIGGER IF EXISTS trg_variant_options_axis_price_modifier
    ON inventory.product_variant_options;
DROP FUNCTION IF EXISTS inventory.fn_reject_axis_option_price_modifier();

ALTER TABLE inventory.product_variant_groups
    DROP CONSTRAINT IF EXISTS chk_variant_groups_axis_single_choice;
ALTER TABLE inventory.product_variant_groups
    DROP COLUMN IF EXISTS affects_inventory;

DROP TABLE IF EXISTS inventory.product_sku_options;
DROP TABLE IF EXISTS inventory.product_skus;

DROP INDEX IF EXISTS inventory.idx_variant_options_id_group_id;
DROP INDEX IF EXISTS inventory.idx_variant_groups_id_product_id;

COMMENT ON TABLE inventory.product_variant_options IS NULL;
