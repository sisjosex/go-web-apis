-- INV-008 phase 2 — the combination generator.
--
-- Re-runnable by construction, not by inspection: it inserts the combinations
-- that are missing and touches nothing else. That is the same property that made
-- INV-007's backfill safe, and it is the reason the loop skips on
-- combination_key rather than counting rows or comparing timestamps.
--
-- What it never does:
--   * touch the default SKU, its quantity or its is_default flag (INV-007 D4)
--   * delete a combination — an axis leaving the tree does that, through
--     sp_update_product_with_variants and its D3 guard
--   * write a price_modifier other than 0 (INV-007 D3: the axis surcharge lives
--     on the SKU, and this spec has no editor for it yet)
--
-- The caps are parameters rather than constants: they come from InventoryConfig
-- (INVENTORY_MAX_AXES / INVENTORY_MAX_COMBINATIONS, D5) so a tenant-shaped limit
-- stays an env decision, and both are checked before the first INSERT.

DROP FUNCTION IF EXISTS inventory.sp_generate_product_skus(UUID, UUID, INT, INT);
CREATE FUNCTION inventory.sp_generate_product_skus(
    p_tenant_id        UUID,
    p_product_id       UUID,
    p_max_axes         INT DEFAULT 3,
    p_max_combinations INT DEFAULT 100
)
RETURNS TABLE(
    product_id        UUID,
    axis_count        INT,
    combination_count INT,
    created_count     INT,
    existing_count    INT,
    stock_by_variant  BOOLEAN,
    message           TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_product_sku   VARCHAR;
    v_reorder_level DECIMAL;
    v_axis_count    INT;
    v_option_count  INT;
    v_combinations  NUMERIC := 1;
    v_created       INT     := 0;
    v_existing      INT     := 0;
    v_combo         RECORD;
    v_key           TEXT;
    v_sku           VARCHAR;
    v_sku_id        UUID;
    v_constraint    TEXT;
BEGIN
    SELECT p.sku INTO v_product_sku
    FROM inventory.products p
    WHERE p.id        = p_product_id
      AND p.tenant_id = p_tenant_id
      AND p.status    = 'active';

    IF NOT FOUND
    THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- The axes, in the order the product form shows them
    SELECT COUNT(*) INTO v_axis_count
    FROM inventory.product_variant_groups vg
    WHERE vg.product_id = p_product_id
      AND vg.affects_inventory;

    IF v_axis_count = 0
    THEN
        RAISE EXCEPTION 'inventory.skus.no-axes' USING ERRCODE = 'P0001';
    END IF;

    IF v_axis_count > p_max_axes
    THEN
        RAISE EXCEPTION 'inventory.skus.cap-exceeded' USING ERRCODE = 'P0001';
    END IF;

    -- The size of the cartesian product, computed arithmetically so an oversized
    -- product is refused without ever being built. NUMERIC so a pathological
    -- option count cannot overflow into a passing check.
    FOR v_option_count IN
        SELECT COUNT(vo.id)
        FROM inventory.product_variant_groups vg
        LEFT JOIN inventory.product_variant_options vo
               ON vo.variant_group_id = vg.id
              AND vo.is_available
        WHERE vg.product_id = p_product_id
          AND vg.affects_inventory
        GROUP BY vg.id
    LOOP
        v_combinations := v_combinations * v_option_count;
    END LOOP;

    -- An axis with no available option produces nothing to build on, which is
    -- the same dead end as having no axis at all.
    IF v_combinations = 0
    THEN
        RAISE EXCEPTION 'inventory.skus.no-axes' USING ERRCODE = 'P0001';
    END IF;

    IF v_combinations > p_max_combinations
    THEN
        RAISE EXCEPTION 'inventory.skus.cap-exceeded' USING ERRCODE = 'P0001';
    END IF;

    -- New combinations inherit the level the shop configured on the bucket they
    -- come from, so fn_stock_status says something meaningful from row one.
    SELECT ps.reorder_level INTO v_reorder_level
    FROM inventory.product_stock ps
    JOIN inventory.product_skus s ON s.id = ps.sku_id
    WHERE s.product_id = p_product_id
      AND s.is_default;

    v_reorder_level := COALESCE(v_reorder_level, 10);

    BEGIN
        FOR v_combo IN
            WITH RECURSIVE axes AS (
                SELECT
                    vg.id AS group_id,
                    ROW_NUMBER() OVER (ORDER BY vg.sort_order, vg.id) AS axis_no
                FROM inventory.product_variant_groups vg
                WHERE vg.product_id = p_product_id
                  AND vg.affects_inventory
            ),
            combos AS (
                SELECT
                    a.axis_no,
                    ARRAY[vo.id] AS option_ids,
                    LEFT(
                        UPPER(REGEXP_REPLACE(vo.option_name, '[^a-zA-Z0-9]+', '', 'g')),
                        8
                    ) AS code
                FROM axes a
                JOIN inventory.product_variant_options vo
                  ON vo.variant_group_id = a.group_id
                WHERE a.axis_no = 1
                  AND vo.is_available
                UNION ALL
                SELECT
                    a.axis_no,
                    c.option_ids || vo.id,
                    c.code || '-' || LEFT(
                        UPPER(REGEXP_REPLACE(vo.option_name, '[^a-zA-Z0-9]+', '', 'g')),
                        8
                    )
                FROM combos c
                JOIN axes a
                  ON a.axis_no = c.axis_no + 1
                JOIN inventory.product_variant_options vo
                  ON vo.variant_group_id = a.group_id
                WHERE vo.is_available
            )
            SELECT c.option_ids, c.code
            FROM combos c
            WHERE c.axis_no = v_axis_count
            ORDER BY c.code
        LOOP
            -- INV-007 D4's format, and the only place phase 2 writes it. The
            -- trigger from 20260815000001 checks it back at COMMIT.
            SELECT array_to_string(
                ARRAY(
                    SELECT o::TEXT
                    FROM unnest(v_combo.option_ids) AS o
                    ORDER BY o::TEXT
                ),
                ':'
            )
            INTO v_key;

            IF EXISTS (
                SELECT 1
                FROM inventory.product_skus s
                WHERE s.product_id      = p_product_id
                  AND s.combination_key = v_key
            )
            THEN
                v_existing := v_existing + 1;
                CONTINUE;
            END IF;

            -- INV-007 D2b: <products.sku>-<option codes>, policed by
            -- UNIQUE(tenant_id, sku).
            v_sku := v_product_sku || '-' || v_combo.code;

            IF LENGTH(v_sku) > 50
            THEN
                RAISE EXCEPTION 'inventory.skus.sku-collision'
                    USING ERRCODE = 'P0001';
            END IF;

            INSERT INTO inventory.product_skus (
                tenant_id,
                product_id,
                sku,
                price_modifier,
                combination_key,
                is_default,
                status
            )
            VALUES (
                p_tenant_id, p_product_id, v_sku, 0, v_key, FALSE, 'active'
            )
            RETURNING product_skus.id INTO v_sku_id;

            INSERT INTO inventory.product_sku_options (
                sku_id, variant_group_id, option_id, product_id
            )
            SELECT v_sku_id, vo.variant_group_id, vo.id, p_product_id
            FROM inventory.product_variant_options vo
            WHERE vo.id = ANY (v_combo.option_ids);

            INSERT INTO inventory.product_stock (
                product_id,
                sku_id,
                current_quantity,
                reserved_quantity,
                reorder_level,
                status
            )
            VALUES (
                p_product_id,
                v_sku_id,
                0,
                0,
                v_reorder_level,
                inventory.fn_stock_status(0, v_reorder_level)
            );

            v_created := v_created + 1;
        END LOOP;
    EXCEPTION
        WHEN unique_violation THEN
            GET STACKED DIAGNOSTICS v_constraint = CONSTRAINT_NAME;

            -- Two options whose codes collapse to the same code, or a code
            -- already taken by another product's SKU. A named error rather than
            -- a raw constraint violation surfacing as a 500.
            IF v_constraint = 'product_skus_tenant_sku_key'
            THEN
                RAISE EXCEPTION 'inventory.skus.sku-collision'
                    USING ERRCODE = 'P0001';
            ELSE
                RAISE;
            END IF;
    END;

    UPDATE inventory.products p
    SET stock_by_variant = TRUE,
        updated_at       = CURRENT_TIMESTAMP
    WHERE p.id = p_product_id;

    RETURN QUERY SELECT
        p_product_id,
        CAST(v_axis_count AS INT),
        CAST(v_combinations AS INT),
        CAST(v_created AS INT),
        CAST(v_existing AS INT),
        TRUE,
        CAST('Combinations generated' AS TEXT);
END;
$$;
COMMENT ON FUNCTION inventory.sp_generate_product_skus IS
    'Generates the cartesian product of a product''s affects_inventory options: '
    'one product_skus row per missing combination at price_modifier 0, its '
    'product_sku_options rows and a product_stock row at 0. Re-runnable — an '
    'existing combination is skipped, the default SKU is never touched — and '
    'refuses past p_max_axes / p_max_combinations before writing anything (D5).';
