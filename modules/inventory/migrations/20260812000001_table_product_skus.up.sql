-- INV-007 phase 1 — the SKU (sellable combination) half of the model.
--
-- Vocabulary, so the two halves never get confused:
--   product_variant_groups / product_variant_options = the DEFINITION of what a
--       product offers (Size: M, XL — Topping: cheese, bacon).
--   product_skus / product_sku_options               = the sellable COMBINATIONS
--       built out of those definitions, and the only thing stock hangs off.

-- ---------------------------------------------------------------------------
-- product_skus — one row per sellable combination
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS inventory.product_skus (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID          NOT NULL,
    product_id      UUID          NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    sku             VARCHAR(50)   NOT NULL,
    price_modifier  DECIMAL(12,2) NOT NULL DEFAULT 0,
    combination_key TEXT          NOT NULL DEFAULT '',
    is_default      BOOLEAN       NOT NULL DEFAULT FALSE,
    status          VARCHAR(20)   NOT NULL DEFAULT 'active'
                                  CHECK (status IN ('active', 'discontinued')),
    created_at      TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT product_skus_tenant_sku_key UNIQUE (tenant_id, sku),
    CONSTRAINT product_skus_product_combination_key UNIQUE (product_id, combination_key)
);

CREATE INDEX IF NOT EXISTS idx_product_skus_tenant_id ON inventory.product_skus(tenant_id);
CREATE INDEX IF NOT EXISTS idx_product_skus_product_id ON inventory.product_skus(product_id);

-- Exactly one default SKU per product, enforced by Postgres and not by the
-- phase-2 generator remembering to.
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_skus_one_default_per_product
    ON inventory.product_skus(product_id)
    WHERE is_default;

-- Target of the composite FKs added by 20260812000002 and by product_sku_options
-- below: a stock/movement/batch row can never name a SKU of another product.
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_skus_id_product_id
    ON inventory.product_skus(id, product_id);

COMMENT ON TABLE inventory.product_skus IS
    'Sellable combination of a product (Polera-M, Polera-XL); the only entity '
    'stock, movements and batches hang off. NOT the definition of the options a '
    'product offers - that is inventory.product_variant_groups / '
    'inventory.product_variant_options.';
COMMENT ON COLUMN inventory.product_skus.combination_key IS
    'The combination''s option UUIDs, sorted ascending as text, joined by '':''. '
    'Empty string for the default SKU. Format fixed by INV-007 D4.';
COMMENT ON COLUMN inventory.product_skus.is_default IS
    'The product''s unassigned bucket: created by the backfill, never deleted, '
    'and from phase 2 on it holds the quantity not yet redistributed across '
    'real combinations.';
COMMENT ON COLUMN inventory.product_skus.price_modifier IS
    'Authoritative delta over products.base_price for anything sold by SKU '
    '(INV-007 D3). Options of an affects_inventory group keep theirs at 0.';

-- ---------------------------------------------------------------------------
-- Helper unique indexes the composite FKs below require
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX IF NOT EXISTS idx_variant_groups_id_product_id
    ON inventory.product_variant_groups(id, product_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_variant_options_id_group_id
    ON inventory.product_variant_options(id, variant_group_id);

-- ---------------------------------------------------------------------------
-- product_sku_options — which option of each axis composes a combination
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS inventory.product_sku_options (
    sku_id           UUID      NOT NULL,
    variant_group_id UUID      NOT NULL,
    option_id        UUID      NOT NULL,
    product_id       UUID      NOT NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (sku_id, variant_group_id),
    -- The SKU belongs to this product.
    CONSTRAINT product_sku_options_sku_fkey
        FOREIGN KEY (sku_id, product_id)
        REFERENCES inventory.product_skus(id, product_id) ON DELETE CASCADE,
    -- The group belongs to this product.
    CONSTRAINT product_sku_options_group_fkey
        FOREIGN KEY (variant_group_id, product_id)
        REFERENCES inventory.product_variant_groups(id, product_id) ON DELETE CASCADE,
    -- The option belongs to this group.
    CONSTRAINT product_sku_options_option_fkey
        FOREIGN KEY (option_id, variant_group_id)
        REFERENCES inventory.product_variant_options(id, variant_group_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_product_sku_options_option_id
    ON inventory.product_sku_options(option_id);
CREATE INDEX IF NOT EXISTS idx_product_sku_options_product_id
    ON inventory.product_sku_options(product_id);

COMMENT ON TABLE inventory.product_sku_options IS
    'One row per axis of a combination: which option of each affects_inventory '
    'group this SKU is made of. product_id is carried so the composite FKs can '
    'reject an option from another group and a group from another product.';

-- ---------------------------------------------------------------------------
-- affects_inventory — which variant groups are axes rather than menu modifiers
-- ---------------------------------------------------------------------------
ALTER TABLE inventory.product_variant_groups
    ADD COLUMN IF NOT EXISTS affects_inventory BOOLEAN NOT NULL DEFAULT FALSE;

-- An axis contributes exactly one option to every combination, so it must be
-- required and single-select. Modifier groups are unconstrained, as today.
ALTER TABLE inventory.product_variant_groups
    DROP CONSTRAINT IF EXISTS chk_variant_groups_axis_single_choice;
ALTER TABLE inventory.product_variant_groups
    ADD CONSTRAINT chk_variant_groups_axis_single_choice
    CHECK (
        affects_inventory = FALSE
        OR (max_selections = 1 AND is_required = TRUE)
    );

COMMENT ON COLUMN inventory.product_variant_groups.affects_inventory IS
    'TRUE = this group is an inventory axis: every one of its options multiplies '
    'the product''s SKUs. FALSE (the default, and every pre-INV-007 row) = a menu '
    'modifier that changes price and presentation but never stock.';

COMMENT ON TABLE inventory.product_variant_options IS
    'The DEFINITION of one option a product offers (Vanilla, XL, Extra cheese). '
    'NOT a sellable combination and NOT a stock-bearing entity - that is '
    'inventory.product_skus.';

-- ---------------------------------------------------------------------------
-- INV-007 D3 — the price of an axis lives on the SKU, never on the option
-- ---------------------------------------------------------------------------
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
