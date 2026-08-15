-- INV-008 phase 2 — the two schema facts the generator needs before it exists.
--
--   1. products.stock_by_variant, so a client can tell "this product's stock is
--      per combination" from "this product has one bucket" without counting SKUs
--   2. the constraint trigger that closes INV-007's last open invariant:
--      product_skus.combination_key must agree with the product_sku_options rows
--      it claims to summarise
--
-- Re-runnable, like every migration in this module: a half-applied tenant has to
-- survive being migrated again.

-- ---------------------------------------------------------------------------
-- stock_by_variant — is this product's stock tracked per combination
-- ---------------------------------------------------------------------------
ALTER TABLE inventory.products
    ADD COLUMN IF NOT EXISTS stock_by_variant BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN inventory.products.stock_by_variant IS
    'TRUE when the product has at least one affects_inventory variant group, so '
    'its stock lives on generated combinations rather than on the default SKU '
    'alone. Written by inventory.sp_generate_product_skus, which owns it, and '
    'cleared by inventory.sp_update_product_with_variants when the last axis '
    'goes away. Never set by hand.';

-- ---------------------------------------------------------------------------
-- combination_key vs product_sku_options — INV-007's open invariant, closed
-- ---------------------------------------------------------------------------
-- INV-007's Risks left this to "the generator SP, phase 2". It does not have to
-- live there: recomputing the key from the row set costs one deferred constraint
-- trigger and holds for every writer, including a hand-written UPDATE.
--
-- DEFERRABLE INITIALLY DEFERRED on purpose. The generator inserts the SKU with
-- its key and then one option row per axis; an immediate check would fire — and
-- fail — after the first of them. Checked at COMMIT, the intermediate states are
-- invisible. To assert it inside a transaction, SET CONSTRAINTS ALL IMMEDIATE.
CREATE OR REPLACE FUNCTION inventory.fn_check_sku_combination_key()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
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
$$;

COMMENT ON FUNCTION inventory.fn_check_sku_combination_key IS
    'Recomputes a SKU''s combination_key from its product_sku_options rows — the '
    'option UUIDs sorted ascending as text and joined by '':'' (INV-007 D4) — and '
    'rejects a key that disagrees. Silent when the SKU row no longer exists.';

DROP TRIGGER IF EXISTS trg_sku_options_combination_key
    ON inventory.product_sku_options;
CREATE CONSTRAINT TRIGGER trg_sku_options_combination_key
    AFTER INSERT OR UPDATE OR DELETE
    ON inventory.product_sku_options
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION inventory.fn_check_sku_combination_key();
