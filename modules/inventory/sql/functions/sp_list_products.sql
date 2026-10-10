CREATE OR REPLACE FUNCTION inventory.sp_list_products(p_tenant_id uuid, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_category_id uuid DEFAULT NULL::uuid, p_search character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, sku character varying, name character varying, description text, base_price numeric, has_variants boolean, status character varying, created_at timestamp without time zone, categories jsonb, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    WITH page AS (
        SELECT
            p.id,
            p.created_at,
            COUNT(*) OVER () AS total_count
        FROM inventory.products p
        WHERE p.tenant_id = p_tenant_id
          AND p.status = 'active'
          AND (
              p_category_id IS NULL
              OR EXISTS (
                  SELECT 1
                  FROM inventory.product_category_mapping fpcm
                  WHERE fpcm.product_id  = p.id
                    AND fpcm.category_id = p_category_id
              )
          )
          AND (
              p_search IS NULL
              OR p_search = ''
              OR p.name ILIKE '%' || p_search || '%'
              OR p.sku  ILIKE '%' || p_search || '%'
          )
        -- p.id breaks ties: products created in one import share created_at, and a
        -- tie without it reshuffles rows between pages.
        ORDER BY p.created_at DESC, p.id DESC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
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
        CAST(pg.total_count AS BIGINT)
    FROM page pg
    INNER JOIN inventory.products p
        ON p.id = pg.id
       AND p.tenant_id = p_tenant_id
    ORDER BY pg.created_at DESC, pg.id DESC;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_products(uuid,integer,integer,uuid,character varying) IS 'One page of a tenant''s active products, newest first, optionally narrowed to a category and a name/SKU search; each row carries its categories and the total_count the filters match';
