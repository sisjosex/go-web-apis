-- Reverses 20260815000002: both variant SPs go back to their 20260808000001 /
-- 20260808000002 definitions, verbatim, and the D3 helper is dropped.

-- ---------------------------------------------------------------------------
-- sp_get_product_with_variants — as of 20260808000001
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION inventory.sp_get_product_with_variants(
    p_tenant_id  UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id           UUID,
    sku          VARCHAR,
    name         VARCHAR,
    description  TEXT,
    base_price   DECIMAL,
    has_variants BOOLEAN,
    status       VARCHAR,
    created_at   TIMESTAMP,
    media        JSONB,
    variants     JSONB
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        p.id,
        p.sku,
        p.name,
        p.description,
        p.base_price,
        p.has_variants,
        p.status,
        p.created_at,

        -- Product-level media (no variant option)
        (
            SELECT COALESCE(
                jsonb_agg(
                    jsonb_build_object(
                        'id',         pm.id,
                        'media_type', pm.media_type,
                        'url',        pm.url,
                        'alt_text',   pm.alt_text,
                        'is_primary', pm.is_primary,
                        'sort_order', pm.sort_order
                    )
                    ORDER BY pm.sort_order, pm.is_primary DESC
                ),
                '[]'::jsonb
            )
            FROM inventory.product_media pm
            WHERE pm.product_id = p.id AND pm.variant_option_id IS NULL
        ) AS media,

        -- Variant groups with options (each option includes its own media)
        CASE
            WHEN p.has_variants THEN (
                SELECT COALESCE(
                    jsonb_agg(
                        jsonb_build_object(
                            'id',             vg.id,
                            'group_type',     vg.group_type,
                            'is_required',    vg.is_required,
                            'max_selections', vg.max_selections,
                            'sort_order',     vg.sort_order,
                            'options', (
                                SELECT COALESCE(
                                    jsonb_agg(
                                        jsonb_build_object(
                                            'id',             vo.id,
                                            'name',           vo.option_name,
                                            'price_modifier', vo.price_modifier,
                                            'is_available',   vo.is_available,
                                            'sort_order',     vo.sort_order,
                                            'media', (
                                                SELECT COALESCE(
                                                    jsonb_agg(
                                                        jsonb_build_object(
                                                            'id',         opm.id,
                                                            'media_type', opm.media_type,
                                                            'url',        opm.url,
                                                            'alt_text',   opm.alt_text,
                                                            'is_primary', opm.is_primary,
                                                            'sort_order', opm.sort_order
                                                        )
                                                        ORDER BY opm.sort_order, opm.is_primary DESC
                                                    ),
                                                    '[]'::jsonb
                                                )
                                                FROM inventory.product_media opm
                                                WHERE opm.variant_option_id = vo.id
                                            )
                                        )
                                        ORDER BY vo.sort_order, vo.option_name
                                    ),
                                    '[]'::jsonb
                                )
                                FROM inventory.product_variant_options vo
                                WHERE vo.variant_group_id = vg.id AND vo.is_available = true
                            )
                        )
                        ORDER BY vg.sort_order, vg.group_type
                    ),
                    '[]'::jsonb
                )
                FROM inventory.product_variant_groups vg
                WHERE vg.product_id = p.id
            )
            ELSE '[]'::jsonb
        END AS variants

    FROM inventory.products p
    WHERE p.tenant_id = p_tenant_id
      AND p.id        = p_product_id
      AND p.status    = 'active';
END;
$$;
COMMENT ON FUNCTION inventory.sp_get_product_with_variants IS
    'Returns a product with its media and its variant tree; groups and options carry their id';

-- ---------------------------------------------------------------------------
-- sp_update_product_with_variants — as of 20260808000002
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION inventory.sp_update_product_with_variants(
    p_tenant_id   UUID,
    p_product_id  UUID,
    p_name        VARCHAR,
    p_description TEXT,
    p_base_price  DECIMAL,
    p_variants    JSONB DEFAULT NULL
)
RETURNS TABLE(
    product_id UUID,
    sku        VARCHAR,
    name       VARCHAR,
    message    TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    l_sku             VARCHAR;
    l_groups          JSONB;
    l_group           JSONB;
    l_options         JSONB;
    l_option          JSONB;
    l_group_id        UUID;
    l_group_idx       INT;
    l_option_idx      INT;
    l_kept_group_ids  UUID[];
    l_kept_option_ids UUID[];
    l_group_count     INT;
    l_constraint      TEXT;
BEGIN
    -- The product must exist, belong to the tenant and still be active
    SELECT p.sku INTO l_sku
    FROM inventory.products p
    WHERE p.id        = p_product_id
      AND p.tenant_id = p_tenant_id
      AND p.status    = 'active';

    IF NOT FOUND
    THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE inventory.products p
    SET name        = p_name,
        description = p_description,
        base_price  = p_base_price,
        updated_at  = CURRENT_TIMESTAMP
    WHERE p.id = p_product_id;

    IF p_variants IS NOT NULL
    THEN
        BEGIN
            -- A missing, null or non-array 'groups' key means "no groups"
            l_groups := CASE
                WHEN jsonb_typeof(p_variants -> 'groups') = 'array'
                THEN p_variants -> 'groups'
                ELSE '[]'::jsonb
            END;

            -- Pass 1: every group id in the payload must belong to this product
            l_kept_group_ids := ARRAY[]::UUID[];

            FOR l_group IN SELECT jsonb_array_elements(l_groups)
            LOOP
                IF l_group ->> 'id' IS NOT NULL
                THEN
                    IF NOT EXISTS (
                        SELECT 1
                        FROM inventory.product_variant_groups vg
                        WHERE vg.id         = (l_group ->> 'id')::UUID
                          AND vg.product_id = p_product_id
                    )
                    THEN
                        RAISE EXCEPTION 'product.variant-group.not-in-product'
                            USING ERRCODE = 'P0001';
                    END IF;

                    l_kept_group_ids := array_append(
                        l_kept_group_ids, (l_group ->> 'id')::UUID
                    );
                END IF;
            END LOOP;

            -- Groups the payload dropped. Cascades to their options and media.
            DELETE FROM inventory.product_variant_groups vg
            WHERE vg.product_id = p_product_id
              AND NOT (vg.id = ANY(l_kept_group_ids));

            -- Pass 2: update the matched groups, insert the new ones,
            -- sort_order follows the array position
            l_group_idx := 0;

            FOR l_group IN SELECT jsonb_array_elements(l_groups)
            LOOP
                IF l_group ->> 'id' IS NOT NULL
                THEN
                    l_group_id := (l_group ->> 'id')::UUID;

                    UPDATE inventory.product_variant_groups vg
                    SET group_type     = l_group ->> 'group_type',
                        is_required    = COALESCE(
                            (l_group ->> 'is_required')::BOOLEAN, FALSE
                        ),
                        max_selections = COALESCE(
                            (l_group ->> 'max_selections')::INT, 1
                        ),
                        sort_order     = l_group_idx
                    WHERE vg.id = l_group_id;
                ELSE
                    INSERT INTO inventory.product_variant_groups (
                        product_id,
                        group_type,
                        is_required,
                        max_selections,
                        sort_order
                    )
                    VALUES (
                        p_product_id,
                        l_group ->> 'group_type',
                        COALESCE((l_group ->> 'is_required')::BOOLEAN, FALSE),
                        COALESCE((l_group ->> 'max_selections')::INT, 1),
                        l_group_idx
                    )
                    RETURNING product_variant_groups.id INTO l_group_id;
                END IF;

                l_options := CASE
                    WHEN jsonb_typeof(l_group -> 'options') = 'array'
                    THEN l_group -> 'options'
                    ELSE '[]'::jsonb
                END;

                -- Every option id in this group must belong to this group
                l_kept_option_ids := ARRAY[]::UUID[];

                FOR l_option IN SELECT jsonb_array_elements(l_options)
                LOOP
                    IF l_option ->> 'id' IS NOT NULL
                    THEN
                        IF NOT EXISTS (
                            SELECT 1
                            FROM inventory.product_variant_options vo
                            WHERE vo.id               = (l_option ->> 'id')::UUID
                              AND vo.variant_group_id = l_group_id
                        )
                        THEN
                            RAISE EXCEPTION 'product.variant-option.not-in-group'
                                USING ERRCODE = 'P0001';
                        END IF;

                        l_kept_option_ids := array_append(
                            l_kept_option_ids, (l_option ->> 'id')::UUID
                        );
                    END IF;
                END LOOP;

                -- Options the payload dropped. Cascades to their media (D3).
                DELETE FROM inventory.product_variant_options vo
                WHERE vo.variant_group_id = l_group_id
                  AND NOT (vo.id = ANY(l_kept_option_ids));

                l_option_idx := 0;

                FOR l_option IN SELECT jsonb_array_elements(l_options)
                LOOP
                    IF l_option ->> 'id' IS NOT NULL
                    THEN
                        UPDATE inventory.product_variant_options vo
                        SET option_name    = l_option ->> 'name',
                            price_modifier = COALESCE(
                                (l_option ->> 'modifier')::DECIMAL, 0
                            ),
                            sort_order     = l_option_idx
                        WHERE vo.id = (l_option ->> 'id')::UUID;
                    ELSE
                        INSERT INTO inventory.product_variant_options (
                            variant_group_id,
                            option_name,
                            price_modifier,
                            sort_order
                        )
                        VALUES (
                            l_group_id,
                            l_option ->> 'name',
                            COALESCE((l_option ->> 'modifier')::DECIMAL, 0),
                            l_option_idx
                        );
                    END IF;

                    l_option_idx := l_option_idx + 1;
                END LOOP;

                l_group_idx := l_group_idx + 1;
            END LOOP;
        EXCEPTION
            WHEN unique_violation THEN
                GET STACKED DIAGNOSTICS l_constraint = CONSTRAINT_NAME;

                IF l_constraint = 'idx_variant_groups_product_type'
                THEN
                    RAISE EXCEPTION 'product.variant-group.duplicate-type'
                        USING ERRCODE = 'P0001';
                ELSIF l_constraint = 'idx_variant_options_group_name'
                THEN
                    RAISE EXCEPTION 'product.variant-option.duplicate-name'
                        USING ERRCODE = 'P0001';
                ELSE
                    RAISE;
                END IF;
        END;

        -- has_variants follows the group count that actually survived
        SELECT COUNT(*) INTO l_group_count
        FROM inventory.product_variant_groups vg
        WHERE vg.product_id = p_product_id;

        UPDATE inventory.products p
        SET has_variants = (l_group_count > 0)
        WHERE p.id = p_product_id;
    END IF;

    RETURN QUERY SELECT
        p_product_id,
        CAST(l_sku AS VARCHAR),
        CAST(p_name AS VARCHAR),
        CAST(
            CASE
                WHEN p_variants IS NOT NULL
                THEN 'Product updated with variants'
                ELSE 'Product updated'
            END AS TEXT
        );
END;
$$;
COMMENT ON FUNCTION inventory.sp_update_product_with_variants IS
    'Updates a product and diffs its variant tree by id; NULL p_variants leaves the tree untouched';

DROP FUNCTION IF EXISTS inventory.fn_purge_skus_for_options(UUID, UUID[]);
