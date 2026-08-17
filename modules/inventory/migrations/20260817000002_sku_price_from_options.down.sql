-- Reverses 20260817000002: the derivation out, INV-007 D3's rejection back in.
--
-- The axis options that carry a modifier under D1 have to be flattened before
-- the rejection can be reinstalled, otherwise the first UPDATE of any of them
-- raises. The SKU modifiers go back to 0 with them, which is exactly what
-- sp_generate_product_skus wrote before this migration.

DROP TRIGGER IF EXISTS trg_variant_options_price_to_skus
    ON inventory.product_variant_options;
DROP TRIGGER IF EXISTS trg_sku_options_price_from_options
    ON inventory.product_sku_options;
DROP FUNCTION IF EXISTS inventory.fn_recalc_sku_price_from_options();

UPDATE inventory.product_skus s
SET price_modifier = 0
WHERE NOT s.is_default;

UPDATE inventory.product_variant_options vo
SET price_modifier = 0
FROM inventory.product_variant_groups vg
WHERE vg.id = vo.variant_group_id
  AND vg.affects_inventory
  AND vo.price_modifier <> 0;

CREATE OR REPLACE FUNCTION inventory.fn_reject_axis_option_price_modifier()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF COALESCE(NEW.price_modifier, 0) <> 0
       AND EXISTS (
            SELECT 1
            FROM inventory.product_variant_groups g
            WHERE g.id = NEW.variant_group_id
              AND g.affects_inventory
       )
    THEN
        RAISE EXCEPTION 'inventory.variant-option.axis-price-modifier-not-allowed'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN NEW;
END;
$$;

COMMENT ON FUNCTION inventory.fn_reject_axis_option_price_modifier IS
    'Keeps price_modifier at 0 on options of an affects_inventory group so the '
    'axis surcharge cannot be charged twice (INV-007 D3). Inert in phase 1: no '
    'axis group exists yet.';

CREATE OR REPLACE TRIGGER trg_variant_options_axis_price_modifier
    BEFORE INSERT OR UPDATE OF price_modifier, variant_group_id
    ON inventory.product_variant_options
    FOR EACH ROW
    EXECUTE FUNCTION inventory.fn_reject_axis_option_price_modifier();

COMMENT ON COLUMN inventory.product_skus.price_modifier IS
    'Authoritative delta over products.base_price for anything sold by SKU '
    '(INV-007 D3). Options of an affects_inventory group keep theirs at 0.';
