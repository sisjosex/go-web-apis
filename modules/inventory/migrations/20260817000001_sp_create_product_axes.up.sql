-- INV-012 D2 (Alternative) — a product declared with an axis arrives complete.
--
-- Three things were wrong with creating a product that has an inventory axis:
--
--   1. sp_create_product_with_variants never read 'affects_inventory'. Every
--      group it wrote was a menu modifier — its own COMMENT said "until phase 2",
--      and phase 2 (20260815000002) only ever taught the *update* SP. So the
--      generator call that the client made a moment later found no axis and
--      raised inventory.skus.no-axes; re-opening the product and saving ran the
--      update SP, which is why the second edit worked and the first did not.
--   2. It had no unique_violation / check_violation handler, so a duplicate
--      group type or an axis that is not a single required choice surfaced as a
--      raw constraint violation — a 500 — where the update SP returns a name.
--   3. Generation was a second round trip. D2 resolves it inline: a payload that
--      declares an axis leaves this SP with its combinations already built.
--
-- Point 3 is why this is DROP + CREATE rather than CREATE OR REPLACE. The caps
-- live in InventoryConfig (INVENTORY_MAX_AXES / INVENTORY_MAX_COMBINATIONS,
-- INV-008 D5) and the SP is the only place that can enforce them, so they have
-- to be passed in. Both are appended LAST with the same defaults
-- sp_generate_product_skus declares, so every positional call site keeps meaning
-- what it means today.
--
-- The body below restates the whole current function (20260812000004), not the
-- 2026-03 one: the default-SKU creation that 20260812000004 added is what every
-- product's NOT NULL sku_id hangs off, and losing it here would break creation
-- outright.

DROP FUNCTION IF EXISTS inventory.sp_create_product_with_variants(
    UUID, VARCHAR, VARCHAR, TEXT, DECIMAL, JSONB, UUID
);
DROP FUNCTION IF EXISTS inventory.sp_create_product_with_variants(
    UUID, VARCHAR, VARCHAR, TEXT, DECIMAL, JSONB, UUID, INT, INT
);
CREATE FUNCTION inventory.sp_create_product_with_variants(
    p_tenant_id        UUID,
    p_sku              VARCHAR,
    p_name             VARCHAR,
    p_description      TEXT,
    p_base_price       DECIMAL,
    p_variants         JSONB DEFAULT NULL,
    p_created_by       UUID  DEFAULT NULL,
    p_max_axes         INT   DEFAULT 3,
    p_max_combinations INT   DEFAULT 100
)
RETURNS TABLE(
    product_id UUID,
    sku        VARCHAR,
    name       VARCHAR,
    message    TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_product_id UUID;
    v_sku_id     UUID;
    v_group      JSONB;
    v_option     JSONB;
    v_group_id   UUID;
    v_group_idx  INT;
    v_option_idx INT;
    v_has_axis   BOOLEAN;
    v_constraint TEXT;
BEGIN
    -- Validate SKU unique per tenant
    IF EXISTS (SELECT 1 FROM inventory.products p WHERE p.sku = p_sku AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.sku.already-exists' USING ERRCODE = 'P0001';
    END IF;

    -- Create product
    INSERT INTO inventory.products (tenant_id, sku, name, description, base_price, has_variants)
    VALUES (p_tenant_id, p_sku, p_name, p_description, p_base_price, CASE WHEN p_variants IS NOT NULL THEN TRUE ELSE FALSE END)
    RETURNING products.id INTO v_product_id;

    -- Create the default SKU: products.sku verbatim, empty combination (D2b/D4)
    INSERT INTO inventory.product_skus (tenant_id, product_id, sku, combination_key, is_default)
    VALUES (p_tenant_id, v_product_id, p_sku, '', TRUE)
    RETURNING product_skus.id INTO v_sku_id;

    -- Create initial stock, hanging off the SKU
    INSERT INTO inventory.product_stock (product_id, sku_id, current_quantity)
    VALUES (v_product_id, v_sku_id, 0);

    UPDATE inventory.product_stock ps
    SET status = inventory.fn_stock_status(ps.current_quantity, ps.reorder_level)
    WHERE ps.sku_id = v_sku_id;

    -- Process variants if provided
    IF p_variants IS NOT NULL THEN
        -- The same two handlers sp_update_product_with_variants carries, for the
        -- same reason: a duplicate group type, a duplicate option name and an
        -- axis that is not a single required choice are all things the client
        -- can fix, and all of them arrived here as a 500.
        BEGIN
            v_group_idx := 0;

            FOR v_group IN SELECT jsonb_array_elements(p_variants -> 'groups')
            LOOP
                INSERT INTO inventory.product_variant_groups (
                    product_id,
                    group_type,
                    is_required,
                    max_selections,
                    affects_inventory,
                    sort_order
                )
                VALUES (
                    v_product_id,
                    v_group->>'group_type',
                    COALESCE((v_group->>'is_required')::BOOLEAN, FALSE),
                    COALESCE((v_group->>'max_selections')::INT, 1),
                    -- INV-012: an omitted key means FALSE, exactly as the update
                    -- SP reads it, so a pre-INV-008 client keeps creating menu
                    -- modifiers and nothing else changes for it.
                    COALESCE((v_group->>'affects_inventory')::BOOLEAN, FALSE),
                    v_group_idx
                )
                RETURNING product_variant_groups.id INTO v_group_id;

                v_option_idx := 0;

                FOR v_option IN SELECT jsonb_array_elements(v_group -> 'options')
                LOOP
                    INSERT INTO inventory.product_variant_options (
                        variant_group_id,
                        option_name,
                        price_modifier,
                        sort_order
                    )
                    VALUES (
                        v_group_id,
                        v_option->>'name',
                        COALESCE((v_option->>'modifier')::DECIMAL, 0),
                        v_option_idx
                    );

                    v_option_idx := v_option_idx + 1;
                END LOOP;

                v_group_idx := v_group_idx + 1;
            END LOOP;
        EXCEPTION
            WHEN unique_violation THEN
                GET STACKED DIAGNOSTICS v_constraint = CONSTRAINT_NAME;

                IF v_constraint = 'idx_variant_groups_product_type'
                THEN
                    RAISE EXCEPTION 'product.variant-group.duplicate-type'
                        USING ERRCODE = 'P0001';
                ELSIF v_constraint = 'idx_variant_options_group_name'
                THEN
                    RAISE EXCEPTION 'product.variant-option.duplicate-name'
                        USING ERRCODE = 'P0001';
                ELSE
                    RAISE;
                END IF;
            WHEN check_violation THEN
                GET STACKED DIAGNOSTICS v_constraint = CONSTRAINT_NAME;

                -- An axis contributes exactly one option to every combination,
                -- so INV-007 pinned it to max_selections = 1 and is_required.
                IF v_constraint = 'chk_variant_groups_axis_single_choice'
                THEN
                    RAISE EXCEPTION 'product.variant-group.axis-not-single-choice'
                        USING ERRCODE = 'P0001';
                ELSE
                    RAISE;
                END IF;
        END;

        -- D2 — the combinations are part of creating the product, not a second
        -- call the client has to remember. Guarded by the axis check because the
        -- generator (rightly) refuses a product that has none: here that is the
        -- ordinary case, a product of pure menu modifiers.
        --
        -- Deliberately outside the block above: the generator raises its own
        -- named errors (inventory.skus.cap-exceeded, inventory.skus.sku-collision)
        -- and they must not be re-read as a constraint name.
        SELECT EXISTS (
            SELECT 1
            FROM inventory.product_variant_groups vg
            WHERE vg.product_id = v_product_id
              AND vg.affects_inventory
        )
        INTO v_has_axis;

        IF v_has_axis
        THEN
            PERFORM inventory.sp_generate_product_skus(
                p_tenant_id, v_product_id, p_max_axes, p_max_combinations
            );
        END IF;
    END IF;

    -- Return with explicit casting
    RETURN QUERY SELECT
        v_product_id,
        CAST(p_sku AS VARCHAR),
        CAST(p_name AS VARCHAR),
        CAST(CASE WHEN p_variants IS NOT NULL THEN 'Product created with variants' ELSE 'Product created' END AS TEXT);
END;
$$;
COMMENT ON FUNCTION inventory.sp_create_product_with_variants IS
    'Creates a product, its default SKU (products.sku verbatim, combination_key '
    '''''), its stock row at 0 hanging off that SKU, and any variant groups and '
    'options supplied. A group carries affects_inventory (absent means FALSE), '
    'and a product that declares at least one axis leaves this SP with its '
    'combinations already generated, within p_max_axes / p_max_combinations '
    '(INV-012 D2). Duplicate group types, duplicate option names and an axis '
    'that is not a single required choice come back named, not raw.';
