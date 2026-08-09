-- sp_get_categories_by_product: Get categories a product is assigned to
-- Mirrors inventory.sp_get_products_by_category from the opposite side.

CREATE OR REPLACE FUNCTION inventory.sp_get_categories_by_product(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_limit      INTEGER DEFAULT 100,
    p_offset     INTEGER DEFAULT 0
)
RETURNS TABLE (
    id            UUID,
    parent_id     UUID,
    name          VARCHAR,
    slug          VARCHAR,
    description   TEXT,
    icon_url      VARCHAR,
    display_order INTEGER,
    is_active     BOOLEAN,
    product_count BIGINT
) LANGUAGE plpgsql AS $$
BEGIN
    -- Check product exists and belongs to tenant
    IF NOT EXISTS (
        SELECT 1 FROM inventory.products pr
        WHERE pr.id = p_product_id AND pr.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(pc.id AS UUID),
        CAST(pc.parent_id AS UUID),
        CAST(pc.name AS VARCHAR),
        CAST(pc.slug AS VARCHAR),
        pc.description,
        CAST(pc.icon_url AS VARCHAR),
        pc.display_order,
        pc.is_active,
        CAST(COUNT(DISTINCT all_pcm.product_id) AS BIGINT)
    FROM inventory.product_categories pc
    INNER JOIN inventory.product_category_mapping pcm
        ON pcm.category_id = pc.id AND pcm.product_id = p_product_id
    LEFT JOIN inventory.product_category_mapping all_pcm
        ON all_pcm.category_id = pc.id
    WHERE pc.tenant_id = p_tenant_id
        AND pc.deleted_at IS NULL
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url,
             pc.display_order, pc.is_active
    ORDER BY pc.display_order ASC, pc.name ASC
    LIMIT p_limit OFFSET p_offset;
END;
$$;

COMMENT ON FUNCTION inventory.sp_get_categories_by_product IS 'Lists the categories a product is assigned to, scoped to a tenant';
