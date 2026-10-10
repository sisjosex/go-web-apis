CREATE OR REPLACE FUNCTION inventory.fn_check_sku_combination_key()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_sku_ids  UUID[];
    v_sku_id   UUID;
    v_declared TEXT;
    v_actual   TEXT;
BEGIN
    -- An UPDATE that moves an option row between SKUs invalidates both keys.
    IF TG_OP = 'INSERT'
    THEN
        v_sku_ids := ARRAY[NEW.sku_id];
    ELSIF TG_OP = 'DELETE'
    THEN
        v_sku_ids := ARRAY[OLD.sku_id];
    ELSE
        v_sku_ids := ARRAY[OLD.sku_id, NEW.sku_id];
    END IF;

    FOREACH v_sku_id IN ARRAY v_sku_ids
    LOOP
        SELECT s.combination_key
        INTO v_declared
        FROM inventory.product_skus s
        WHERE s.id = v_sku_id;

        -- The SKU itself is gone (deleted, or cascaded away with its product or
        -- its axis): there is no key left for the rows to disagree with.
        IF FOUND
        THEN
            SELECT COALESCE(
                string_agg(so.option_id::TEXT, ':' ORDER BY so.option_id::TEXT),
                ''
            )
            INTO v_actual
            FROM inventory.product_sku_options so
            WHERE so.sku_id = v_sku_id;

            IF v_actual IS DISTINCT FROM v_declared
            THEN
                RAISE EXCEPTION 'inventory.sku.combination-key-mismatch'
                    USING ERRCODE = 'P0001';
            END IF;
        END IF;
    END LOOP;

    RETURN NULL;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_check_sku_combination_key() IS 'Recomputes a SKU''s combination_key from its product_sku_options rows — the option UUIDs sorted ascending as text and joined by '':'' (INV-007 D4) — and rejects a key that disagrees. Silent when the SKU row no longer exists.';

DROP TRIGGER IF EXISTS trg_sku_options_combination_key ON inventory.product_sku_options;
CREATE CONSTRAINT TRIGGER trg_sku_options_combination_key AFTER INSERT OR DELETE OR UPDATE ON inventory.product_sku_options DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION inventory.fn_check_sku_combination_key();
