-- Drop first because return type changes (PostgreSQL does not allow CREATE OR REPLACE to change return type)
DROP FUNCTION IF EXISTS inventory.sp_get_product_with_variants(UUID, UUID);

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
