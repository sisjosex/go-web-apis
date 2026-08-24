-- INV-016 D2 — one named combination, not the cartesian product.
--
-- sp_generate_product_skus builds every combination the axes span and cannot be
-- asked for one of them, which is exactly what a catalogue file needs: the
-- REMERA-BAS sheet lists 3 of the 6 that talle{M,L,XL} x color{Negro,Blanco}
-- spans. This is that SP's loop body with the cartesian product replaced by the
-- single combination the row names, and with the axis and the option created on
-- the way in when the file is the first thing to mention them.
--
-- What it keeps from the generator, so a catalogue built here and one built
-- through the UI stay on one convention (D6):
--   * the derived code — UPPER, non-alphanumerics stripped, 8 chars per option,
--     joined by '-' behind the product's own SKU — applied when p_sku is NULL
--   * the axis order that code is derived in: (sort_order, id), the order the
--     product form shows
--   * a product_stock row at 0 carrying the default bucket's reorder level, and
--     products.stock_by_variant = TRUE
--
-- What it never does: touch the default SKU, update an axis, an option or a
-- combination that already exists, or write product_skus.price_modifier —
-- fn_recalc_sku_price_from_options derives that from the options composing the
-- combination (INV-012 D1). The price a catalogue row carries therefore arrives
-- as the option's own modifier in p_axes and is applied only when the option is
-- created (D3): an option the shop already priced is never repriced by an
-- import.
--
-- p_axes is the row's axis columns in file order:
--   [{"axis": "Talle", "option": "M", "modifier": 0}, …]
-- The caller has already checked the marker, the option set and the modifier
-- (D1/D3); what is enforced here is what only the database can see — the
-- product exists, the axis cap holds, and the combination is new.

DROP FUNCTION IF EXISTS inventory.sp_create_product_sku_combination(
    UUID, UUID, JSONB, VARCHAR, INT
);
CREATE FUNCTION inventory.sp_create_product_sku_combination(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_axes       JSONB,
    p_sku        VARCHAR DEFAULT NULL,
    p_max_axes   INT     DEFAULT 3
)
RETURNS TABLE(
    product_id UUID,
    sku_id     UUID,
    sku        VARCHAR,
    axis_count INT
) LANGUAGE plpgsql AS $$
DECLARE
    v_product_sku   VARCHAR;
    v_reorder_level DECIMAL;
    v_axis_count    INT;
    v_axis          JSONB;
    v_index         INT := 0;
    v_axis_name     VARCHAR;
    v_option_name   VARCHAR;
    v_modifier      DECIMAL;
    v_group_id      UUID;
    v_option_id     UUID;
    v_option_ids    UUID[] := ARRAY[]::UUID[];
    v_code          TEXT;
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

    v_axis_count := COALESCE(jsonb_array_length(p_axes), 0);

    IF v_axis_count = 0
    THEN
        RAISE EXCEPTION 'inventory.skus.no-axes' USING ERRCODE = 'P0001';
    END IF;

    IF v_axis_count > p_max_axes
    THEN
        RAISE EXCEPTION 'inventory.skus.cap-exceeded' USING ERRCODE = 'P0001';
    END IF;

    -- New combinations inherit the level the shop configured on the bucket they
    -- come from, exactly as the generator does, so fn_stock_status says
    -- something meaningful from row one.
    SELECT ps.reorder_level INTO v_reorder_level
    FROM inventory.product_stock ps
    JOIN inventory.product_skus s ON s.id = ps.sku_id
    WHERE s.product_id = p_product_id
      AND s.is_default;

    v_reorder_level := COALESCE(v_reorder_level, 10);

    FOR v_axis IN SELECT jsonb_array_elements(p_axes)
    LOOP
        v_index       := v_index + 1;
        v_axis_name   := TRIM(COALESCE(v_axis ->> 'axis', ''));
        v_option_name := TRIM(COALESCE(v_axis ->> 'option', ''));
        v_modifier    := COALESCE((v_axis ->> 'modifier')::DECIMAL, 0);

        IF v_axis_name = '' OR v_option_name = ''
        THEN
            RAISE EXCEPTION 'inventory.skus.no-axes' USING ERRCODE = 'P0001';
        END IF;

        -- Only an existing axis is reused. A modifier group that happens to
        -- share the name is not one, and letting it be would hang the
        -- combination off a group sp_generate_product_skus does not walk.
        SELECT vg.id INTO v_group_id
        FROM inventory.product_variant_groups vg
        WHERE vg.product_id = p_product_id
          AND vg.affects_inventory
          AND LOWER(vg.group_type) = LOWER(v_axis_name)
        LIMIT 1;

        IF NOT FOUND
        THEN
            -- chk_variant_groups_axis_single_choice: an axis contributes exactly
            -- one option to every combination, so it is required and
            -- single-select. sort_order is the column's position in the file,
            -- which is the order the derived code then reads the axes in.
            INSERT INTO inventory.product_variant_groups (
                product_id,
                group_type,
                is_required,
                max_selections,
                sort_order,
                affects_inventory
            )
            VALUES (
                p_product_id, v_axis_name, TRUE, 1, v_index, TRUE
            )
            RETURNING product_variant_groups.id INTO v_group_id;
        END IF;

        SELECT vo.id INTO v_option_id
        FROM inventory.product_variant_options vo
        WHERE vo.variant_group_id = v_group_id
          AND LOWER(vo.option_name) = LOWER(v_option_name)
        LIMIT 1;

        IF NOT FOUND
        THEN
            INSERT INTO inventory.product_variant_options (
                variant_group_id,
                option_name,
                price_modifier,
                is_available,
                sort_order
            )
            VALUES (
                v_group_id,
                v_option_name,
                v_modifier,
                TRUE,
                (
                    SELECT COUNT(*)
                    FROM inventory.product_variant_options vo2
                    WHERE vo2.variant_group_id = v_group_id
                )
            )
            RETURNING product_variant_options.id INTO v_option_id;
        END IF;

        v_option_ids := v_option_ids || v_option_id;
    END LOOP;

    -- The generator's own derivation, read in the generator's own axis order,
    -- so a code this SP derives is the code that SP would have derived.
    SELECT string_agg(
               LEFT(
                   UPPER(
                       REGEXP_REPLACE(vo.option_name, '[^a-zA-Z0-9]+', '', 'g')
                   ),
                   8
               ),
               '-' ORDER BY vg.sort_order, vg.id
           )
    INTO v_code
    FROM inventory.product_variant_options vo
    JOIN inventory.product_variant_groups vg ON vg.id = vo.variant_group_id
    WHERE vo.id = ANY (v_option_ids);

    -- INV-007 D4's format, checked back at COMMIT by the trigger from
    -- 20260815000001.
    SELECT array_to_string(
               ARRAY(
                   SELECT o::TEXT
                   FROM unnest(v_option_ids) AS o
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
        RAISE EXCEPTION 'inventory.skus.combination-exists'
            USING ERRCODE = 'P0001';
    END IF;

    -- D6 — a filled variant_sku is the code verbatim, a blank one is derived.
    v_sku := NULLIF(TRIM(COALESCE(p_sku, '')), '');

    IF v_sku IS NULL
    THEN
        v_sku := v_product_sku || '-' || v_code;
    END IF;

    IF LENGTH(v_sku) > 50
    THEN
        RAISE EXCEPTION 'inventory.skus.sku-collision' USING ERRCODE = 'P0001';
    END IF;

    BEGIN
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
        WHERE vo.id = ANY (v_option_ids);

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
    EXCEPTION
        WHEN unique_violation THEN
            GET STACKED DIAGNOSTICS v_constraint = CONSTRAINT_NAME;

            -- A code already taken by another SKU of this tenant, or a
            -- combination another row of the same file already created. Named
            -- errors rather than a raw constraint violation surfacing as a 500.
            IF v_constraint = 'product_skus_tenant_sku_key'
            THEN
                RAISE EXCEPTION 'inventory.skus.sku-collision'
                    USING ERRCODE = 'P0001';
            ELSIF v_constraint = 'product_skus_product_combination_key'
            THEN
                RAISE EXCEPTION 'inventory.skus.combination-exists'
                    USING ERRCODE = 'P0001';
            ELSE
                RAISE;
            END IF;
    END;

    -- has_variants as well as stock_by_variant, and this is the only SP that
    -- has to say so: the generator only ever runs on a tree
    -- sp_create_product_with_variants already built, while a catalogue row
    -- creates the product plain and grows its axes one combination at a time.
    -- Without it sp_get_product_with_media returns no tree at all — its variant
    -- branch reads WHEN p.has_variants — so an imported product would show its
    -- combinations and none of the axes composing them.
    UPDATE inventory.products p
    SET has_variants     = TRUE,
        stock_by_variant = TRUE,
        updated_at       = CURRENT_TIMESTAMP
    WHERE p.id = p_product_id;

    RETURN QUERY SELECT
        p_product_id,
        v_sku_id,
        CAST(v_sku AS VARCHAR),
        CAST(v_axis_count AS INT);
END;
$$;
COMMENT ON FUNCTION inventory.sp_create_product_sku_combination IS
    'Creates exactly the one combination p_axes names on an existing product '
    '(INV-016 D2): the axis and the option are created when missing, then one '
    'product_skus row, its product_sku_options rows and a product_stock row at '
    '0 carrying the default bucket''s reorder level, and '
    'products.has_variants = products.stock_by_variant = TRUE. p_sku NULL '
    'derives the code the way '
    'sp_generate_product_skus does (D6). Never touches the default SKU, never '
    'updates an axis, an option or a combination that already exists, and never '
    'writes price_modifier - fn_recalc_sku_price_from_options derives it.';
