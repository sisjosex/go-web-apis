-- sp_list_products: add p_category_id / p_search filters and a categories JSONB column.
-- Signature AND return type change, so DROP + CREATE — never CREATE OR REPLACE.

DROP FUNCTION IF EXISTS inventory.sp_list_products(UUID, INT, INT);

CREATE FUNCTION inventory.sp_list_products(
    p_tenant_id   UUID,
    p_limit       INT     DEFAULT 20,
    p_offset      INT     DEFAULT 0,
    p_category_id UUID    DEFAULT NULL,
    p_search      VARCHAR DEFAULT NULL
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
    categories   JSONB
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

        -- Every category the product is filed under, independent of p_category_id:
        -- a correlated subquery, so a product in three categories is still one row.
        (
            SELECT COALESCE(
                jsonb_agg(
                    jsonb_build_object(
                        'id',   pc.id,
                        'name', pc.name,
                        'slug', pc.slug
                    )
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
        ) AS categories

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
    ORDER BY p.created_at DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$;

COMMENT ON FUNCTION inventory.sp_list_products IS 'Lists active products for a tenant with each row''s categories, optionally filtered by category and by a name/SKU search';
