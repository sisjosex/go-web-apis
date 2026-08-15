-- Reverse of 20260812000003 — hands back a database the pre-INV-007 binary
-- serves correctly.

-- Refuse rather than cascade phase-2 data away: once real combinations exist,
-- their quantities cannot be folded back into one row per product.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM inventory.product_skus s
        WHERE NOT s.is_default
    )
    THEN
        RAISE EXCEPTION
            'inventory.down.non-default-skus-exist'
            USING ERRCODE = 'P0001',
                  HINT = 'Non-default SKUs hold stock this migration cannot merge back into product_stock. Remove them deliberately first.';
    END IF;
END;
$$;

DROP VIEW IF EXISTS inventory.v_product_stock_totals;

-- Dropping the columns drops the composite FKs with them, so the DELETE below
-- can no longer cascade into product_stock.
ALTER TABLE inventory.product_batches     DROP COLUMN IF EXISTS sku_id;
ALTER TABLE inventory.inventory_movements DROP COLUMN IF EXISTS sku_id;
ALTER TABLE inventory.product_stock       DROP COLUMN IF EXISTS sku_id;

ALTER TABLE inventory.product_stock
    DROP CONSTRAINT IF EXISTS product_stock_sku_id_key;
ALTER TABLE inventory.product_stock
    DROP CONSTRAINT IF EXISTS product_stock_product_id_key;
ALTER TABLE inventory.product_stock
    ADD CONSTRAINT product_stock_product_id_key UNIQUE (product_id);

DELETE FROM inventory.product_skus WHERE is_default;

-- The product_stock rows this migration created at 0 for products that never
-- had one are left in place on purpose: the pre-INV-007 code reads them
-- happily, and deleting them would throw away any quantity added since.
