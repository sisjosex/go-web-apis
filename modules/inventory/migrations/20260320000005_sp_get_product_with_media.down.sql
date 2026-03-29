-- Restore sp_get_product_with_variants without media columns
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
    variants     JSONB
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        p.id, p.sku, p.name, p.description, p.base_price,
        p.has_variants, p.status, p.created_at,
        CASE
            WHEN p.has_variants THEN (
                SELECT COALESCE(
                    jsonb_agg(
                        jsonb_build_object(
                            'group_type', vg.group_type,
                            'is_required', vg.is_required,
                            'max_selections', vg.max_selections,
                            'sort_order', vg.sort_order,
                            'options', (
                                SELECT COALESCE(
                                    jsonb_agg(
                                        jsonb_build_object(
                                            'id', vo.id, 'name', vo.option_name,
                                            'price_modifier', vo.price_modifier,
                                            'is_available', vo.is_available,
                                            'sort_order', vo.sort_order
                                        ) ORDER BY vo.sort_order, vo.option_name
                                    ), '[]'::jsonb
                                )
                                FROM inventory.product_variant_options vo
                                WHERE vo.variant_group_id = vg.id AND vo.is_available = true
                            )
                        ) ORDER BY vg.sort_order, vg.group_type
                    ), '[]'::jsonb
                )
                FROM inventory.product_variant_groups vg WHERE vg.product_id = p.id
            )
            ELSE '[]'::jsonb
        END AS variants
    FROM inventory.products p
    WHERE p.tenant_id = p_tenant_id AND p.id = p_product_id AND p.status = 'active';
END;
$$;
