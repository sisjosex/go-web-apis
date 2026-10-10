CREATE OR REPLACE FUNCTION inventory.fn_recalc_sku_price_from_options()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION inventory.fn_recalc_sku_price_from_options() IS 'Recomputes inventory.product_skus.price_modifier as the sum of the price_modifier of the options composing each combination (INV-012 D1). Statement-level with a transition table, so it holds for a bulk insert or a bulk reprice, not only for the first row. The default SKU composes no option and therefore stays at 0.';

DROP TRIGGER IF EXISTS trg_sku_options_price_from_options ON inventory.product_sku_options;
CREATE TRIGGER trg_sku_options_price_from_options AFTER INSERT ON inventory.product_sku_options REFERENCING NEW TABLE AS new_rows FOR EACH STATEMENT EXECUTE FUNCTION inventory.fn_recalc_sku_price_from_options();

DROP TRIGGER IF EXISTS trg_variant_options_price_to_skus ON inventory.product_variant_options;
CREATE TRIGGER trg_variant_options_price_to_skus AFTER UPDATE ON inventory.product_variant_options REFERENCING OLD TABLE AS old_rows NEW TABLE AS new_rows FOR EACH STATEMENT EXECUTE FUNCTION inventory.fn_recalc_sku_price_from_options();
