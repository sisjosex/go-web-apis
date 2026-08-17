-- INV-012 D1 — an axis option carries its price again, and the SKU derives one.
--
-- INV-007 D3 put the axis surcharge on product_skus.price_modifier and installed
-- trg_variant_options_axis_price_modifier to keep the option's own modifier at 0,
-- so the same money could not be charged twice. The half that never arrived is
-- the editor: sp_generate_product_skus writes the SKU's modifier as 0 with the
-- comment "this spec has no editor for it yet", and no endpoint sets it. The net
-- effect is that a product sold by size cannot charge more for XL anywhere.
--
-- D1 resolves it by derivation rather than by a second editor: the option
-- modifier stops being a competing price and becomes the input the SKU price is
-- computed from. product_skus.price_modifier is the sum of the options composing
-- the combination, and it stays the single authoritative read. The double charge
-- cannot come back while that holds — if a later spec adds a per-combination
-- override, the question is open again and has to be answered then.
--
-- It lives in a trigger for the reason 20260815000001 already gives for
-- fn_check_sku_combination_key: recomputing from the row set holds for every
-- writer, including a hand-written UPDATE, and costs neither 300-line SP a
-- rewrite.
--
-- FOR EACH STATEMENT with transition tables, not FOR EACH ROW: the generator
-- inserts one product_sku_options statement per combination, and repricing an
-- option can move a hundred SKUs in one UPDATE. The recompute is driven off
-- every row the transition table names, never off the first one.
--
-- The reprice trigger is a plain AFTER UPDATE rather than AFTER UPDATE OF
-- price_modifier: Postgres refuses a column list on a trigger that declares
-- transition tables ("transition tables cannot be specified for triggers with
-- column lists"). The narrowing moves into the body instead, where old_rows is
-- joined to new_rows and only a modifier that actually changed recomputes
-- anything — which matters, because sp_update_product_with_variants rewrites
-- every option of the tree on every ordinary product save.

-- ---------------------------------------------------------------------------
-- Out: the rejection
-- ---------------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_variant_options_axis_price_modifier
    ON inventory.product_variant_options;
DROP FUNCTION IF EXISTS inventory.fn_reject_axis_option_price_modifier();

-- ---------------------------------------------------------------------------
-- In: the derivation
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION inventory.fn_recalc_sku_price_from_options()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    -- One function, two triggers, two shapes of transition table. Both declare
    -- it as new_rows; which relation it is comes from the table the statement
    -- ran against.
    IF TG_TABLE_NAME = 'product_sku_options'
    THEN
        -- A combination being composed: the SKUs the new rows belong to.
        UPDATE inventory.product_skus s
        SET price_modifier = COALESCE((
                SELECT SUM(vo.price_modifier)
                FROM inventory.product_sku_options so
                JOIN inventory.product_variant_options vo ON vo.id = so.option_id
                WHERE so.sku_id = s.id
            ), 0),
            updated_at = CURRENT_TIMESTAMP
        WHERE s.id IN (SELECT DISTINCT n.sku_id FROM new_rows n);
    ELSE
        -- The shop repricing an option: every combination built on one whose
        -- modifier actually moved.
        UPDATE inventory.product_skus s
        SET price_modifier = COALESCE((
                SELECT SUM(vo.price_modifier)
                FROM inventory.product_sku_options so
                JOIN inventory.product_variant_options vo ON vo.id = so.option_id
                WHERE so.sku_id = s.id
            ), 0),
            updated_at = CURRENT_TIMESTAMP
        WHERE s.id IN (
            SELECT DISTINCT so.sku_id
            FROM inventory.product_sku_options so
            JOIN new_rows n ON n.id = so.option_id
            JOIN old_rows o ON o.id = n.id
            WHERE n.price_modifier IS DISTINCT FROM o.price_modifier
        );
    END IF;

    RETURN NULL;
END;
$$;

COMMENT ON FUNCTION inventory.fn_recalc_sku_price_from_options IS
    'Recomputes inventory.product_skus.price_modifier as the sum of the '
    'price_modifier of the options composing each combination (INV-012 D1). '
    'Statement-level with a transition table, so it holds for a bulk insert or '
    'a bulk reprice, not only for the first row. The default SKU composes no '
    'option and therefore stays at 0.';

DROP TRIGGER IF EXISTS trg_sku_options_price_from_options
    ON inventory.product_sku_options;
CREATE TRIGGER trg_sku_options_price_from_options
    AFTER INSERT
    ON inventory.product_sku_options
    REFERENCING NEW TABLE AS new_rows
    FOR EACH STATEMENT
    EXECUTE FUNCTION inventory.fn_recalc_sku_price_from_options();

DROP TRIGGER IF EXISTS trg_variant_options_price_to_skus
    ON inventory.product_variant_options;
CREATE TRIGGER trg_variant_options_price_to_skus
    AFTER UPDATE
    ON inventory.product_variant_options
    REFERENCING OLD TABLE AS old_rows NEW TABLE AS new_rows
    FOR EACH STATEMENT
    EXECUTE FUNCTION inventory.fn_recalc_sku_price_from_options();

-- ---------------------------------------------------------------------------
-- The whole backfill (Scope): every combination generated before this migration
-- was written at 0 and now reads what its options already say.
-- ---------------------------------------------------------------------------
UPDATE inventory.product_skus s
SET price_modifier = COALESCE((
        SELECT SUM(vo.price_modifier)
        FROM inventory.product_sku_options so
        JOIN inventory.product_variant_options vo ON vo.id = so.option_id
        WHERE so.sku_id = s.id
    ), 0)
WHERE NOT s.is_default;

COMMENT ON COLUMN inventory.product_skus.price_modifier IS
    'Authoritative delta over products.base_price for anything sold by SKU '
    '(INV-007 D3). Derived, never edited: it is the sum of the price_modifier '
    'of the options composing the combination, maintained by '
    'inventory.fn_recalc_sku_price_from_options (INV-012 D1). 0 on the default '
    'SKU, which composes no option.';
