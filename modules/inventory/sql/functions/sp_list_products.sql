CREATE OR REPLACE FUNCTION inventory.sp_list_products(p_tenant_id uuid, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_category_id uuid DEFAULT NULL::uuid, p_search character varying DEFAULT NULL::character varying, p_status character varying DEFAULT 'active'::character varying, p_filter character varying DEFAULT NULL::character varying, p_attrs jsonb DEFAULT NULL::jsonb)
 RETURNS TABLE(id uuid, sku character varying, name character varying, description text, base_price numeric, has_variants boolean, status character varying, created_at timestamp without time zone, categories jsonb, total_count bigint, available_quantity numeric, stock_status character varying, variants_count integer, axes jsonb, min_price numeric, max_price numeric, primary_image_url text, pos_visible boolean, counts jsonb, units jsonb, attributes jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_filter JSONB := inventory.fn_attribute_filter(p_tenant_id, p_attrs);
BEGIN
    RETURN QUERY
    WITH scope AS (
        SELECT
            p.id,
            p.created_at,
            p.status,
            p.pos_visible,
            st.status AS stock_status,
            EXISTS (
                SELECT 1 FROM inventory.product_skus s
                WHERE s.product_id = p.id AND NOT s.is_default
            ) AS with_variants
        FROM inventory.fn_products_in_scope(p_tenant_id, p_category_id, p_search, v_filter) m
        INNER JOIN inventory.products p ON p.id = m.id
        -- A filter on the view's grouped column reaches the index on product_stock, row by row.
        LEFT JOIN LATERAL (
            SELECT v.status FROM inventory.v_product_stock_totals v WHERE v.product_id = p.id
        ) st ON TRUE
    ),
    chips AS (
        SELECT jsonb_build_object(
            'all', COUNT(*) FILTER (WHERE s.status = 'active'),
            'with_variants', COUNT(*) FILTER (WHERE s.status = 'active' AND s.with_variants),
            'low', COUNT(*) FILTER (WHERE s.status = 'active' AND s.stock_status IN ('low', 'critical')),
            'out_of_stock', COUNT(*) FILTER (WHERE s.status = 'active' AND s.stock_status = 'out_of_stock'),
            'pos_hidden', COUNT(*) FILTER (WHERE s.status = 'active' AND NOT s.pos_visible)
        ) AS counts
        FROM scope s
    ),
    page AS (
        SELECT s.id, s.created_at, COUNT(*) OVER () AS total_count
        FROM scope s
        WHERE (p_status = 'all' OR s.status = p_status)
          AND (
              p_filter IS NULL
              OR p_filter = ''
              OR (p_filter = 'with_variants' AND s.with_variants)
              OR (p_filter = 'low' AND s.stock_status IN ('low', 'critical'))
              OR (p_filter = 'out_of_stock' AND s.stock_status = 'out_of_stock')
              OR (p_filter = 'pos_hidden' AND NOT s.pos_visible)
          )
        -- p.id breaks ties: products created in one import share created_at, and a
        -- tie without it reshuffles rows between pages.
        ORDER BY s.created_at DESC, s.id DESC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    -- One row with a NULL id on an empty page, so the chips keep their counts.
    SELECT
        p.id,
        CAST(p.sku AS VARCHAR),
        CAST(p.name AS VARCHAR),
        p.description,
        p.base_price,
        p.has_variants,
        CAST(p.status AS VARCHAR),
        p.created_at,
        -- Every category the product is filed under, independent of p_category_id.
        (
            SELECT COALESCE(
                jsonb_agg(
                    jsonb_build_object('id', pc.id, 'name', pc.name, 'slug', pc.slug)
                    ORDER BY pc.display_order, pc.name
                ),
                '[]'::jsonb
            )
            FROM inventory.product_category_mapping pcm
            INNER JOIN inventory.product_categories pc
                ON pc.id = pcm.category_id
               AND pc.tenant_id = p_tenant_id
               AND pc.deleted_at IS NULL
            WHERE pcm.product_id = p.id
        ),
        CAST(COALESCE(pg.total_count, 0) AS BIGINT),
        CAST(st.available_quantity AS NUMERIC),
        CAST(st.status AS VARCHAR),
        CAST(sk.variants AS INT),
        (
            SELECT COALESCE(
                jsonb_agg(
                    jsonb_build_object('name', vg.group_type, 'options', (
                        SELECT COALESCE(jsonb_agg(vo.option_name ORDER BY vo.sort_order, vo.option_name), '[]'::jsonb)
                        FROM inventory.product_variant_options vo
                        WHERE vo.variant_group_id = vg.id AND vo.is_available
                    ))
                    ORDER BY vg.sort_order, vg.group_type
                ),
                '[]'::jsonb
            )
            FROM inventory.product_variant_groups vg
            WHERE vg.product_id = p.id AND vg.affects_inventory
        ),
        CAST(p.base_price + sk.min_modifier AS NUMERIC),
        CAST(p.base_price + sk.max_modifier AS NUMERIC),
        (
            SELECT pm.url
            FROM inventory.product_media pm
            WHERE pm.product_id = p.id AND pm.variant_option_id IS NULL AND pm.media_type = 'image'
            ORDER BY pm.is_primary DESC, pm.sort_order, pm.created_at
            LIMIT 1
        ),
        p.pos_visible,
        c.counts,
        CASE WHEN p.id IS NULL THEN '[]'::jsonb ELSE inventory.fn_product_units(p_tenant_id, p.id) END,
        p.attributes
    FROM chips c
    LEFT JOIN page pg ON TRUE
    LEFT JOIN inventory.products p
        ON p.id = pg.id
       AND p.tenant_id = p_tenant_id
    LEFT JOIN LATERAL (
        SELECT v.available_quantity, v.status FROM inventory.v_product_stock_totals v WHERE v.product_id = p.id
    ) st ON TRUE
    LEFT JOIN LATERAL (
        SELECT
            COUNT(*) FILTER (WHERE NOT s.is_default) AS variants,
            COALESCE(MIN(s.price_modifier) FILTER (WHERE NOT s.is_default), 0) AS min_modifier,
            COALESCE(MAX(s.price_modifier) FILTER (WHERE NOT s.is_default), 0) AS max_modifier
        FROM inventory.product_skus s
        WHERE s.product_id = p.id AND s.status = 'active'
    ) sk ON TRUE
    ORDER BY pg.created_at DESC, pg.id DESC;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_products(uuid,integer,integer,uuid,character varying,character varying,character varying,jsonb) IS 'One page of a tenant''s products, newest first, by status (active, discontinued, all), chip (with_variants, low, out_of_stock, pos_hidden), category and name/SKU search; each row carries its categories, stock, axes, price range and first image, and every row the active counts behind the chips, in the same statement. An empty page is one row with a NULL id, so the counts still arrive (INV-019 D2).';
