CREATE OR REPLACE FUNCTION inventory.sp_get_product_with_variants(p_tenant_id uuid, p_product_id uuid)
 RETURNS TABLE(id uuid, sku character varying, name character varying, description text, base_price numeric, has_variants boolean, status character varying, created_at timestamp without time zone, media jsonb, variants jsonb, updated_at timestamp without time zone, updated_by_name character varying, pos_visible boolean, orders_visible boolean, units jsonb, attributes jsonb)
 LANGUAGE plpgsql
AS $function$
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
                            'id',                vg.id,
                            'attribute_id',      vg.attribute_id,
                            'group_type',        vg.group_type,
                            'is_required',       vg.is_required,
                            'max_selections',    vg.max_selections,
                            'sort_order',        vg.sort_order,
                            'affects_inventory', vg.affects_inventory,
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
        END AS variants,

        p.updated_at,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR),
        p.pos_visible,
        p.orders_visible,
        inventory.fn_product_units(p_tenant_id, p.id),
        p.attributes

    FROM inventory.products p
    LEFT JOIN auth.users u ON u.id = p.updated_by
    WHERE p.tenant_id = p_tenant_id
      AND p.id        = p_product_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_product_with_variants(uuid,uuid) IS 'Returns a product with its media and its variant tree; groups carry their id and affects_inventory, so a client can tell an inventory axis from a menu modifier without a second call; updated_by_name is who last changed it (INV-020 D1)';
