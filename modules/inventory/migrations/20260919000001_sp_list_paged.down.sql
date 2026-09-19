-- Restores the three list SPs as they stood before APP-004: sp_list_products on
-- limit/offset, the two batch lists unpaged, none with a total_count.

DROP INDEX IF EXISTS inventory.idx_product_batches_tenant_open_expiry;
DROP INDEX IF EXISTS inventory.idx_products_sku_trgm;
DROP INDEX IF EXISTS inventory.idx_products_name_trgm;
DROP INDEX IF EXISTS inventory.idx_products_tenant_active_created;

DROP FUNCTION IF EXISTS inventory.sp_list_products(UUID, INT, INT, UUID, VARCHAR);
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

DROP FUNCTION IF EXISTS inventory.sp_list_batches_by_product(UUID, UUID, BOOLEAN, UUID, INT, INT);
CREATE FUNCTION inventory.sp_list_batches_by_product(
    p_tenant_id   UUID,
    p_product_id  UUID,
    p_only_active BOOLEAN DEFAULT TRUE,
    p_sku_id      UUID DEFAULT NULL
)
RETURNS TABLE(
    id               UUID,
    product_id       UUID,
    lot_number       VARCHAR,
    purchase_date    DATE,
    expiry_date      DATE,
    unit_cost        DECIMAL,
    initial_quantity DECIMAL,
    current_quantity DECIMAL,
    status           VARCHAR,
    days_to_expiry   INT,
    sku_id           UUID,
    sku              VARCHAR
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(pb.status AS VARCHAR),
        CAST(pb.expiry_date - CURRENT_DATE AS INT),
        pb.sku_id,
        CAST(s.sku AS VARCHAR)
    FROM inventory.product_batches pb
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.tenant_id = p_tenant_id
        AND pb.product_id = p_product_id
        AND (p_sku_id IS NULL OR pb.sku_id = p_sku_id)
        AND (NOT p_only_active OR pb.status NOT IN ('expired', 'void'))
    ORDER BY pb.expiry_date ASC, pb.created_at ASC;
END;
$$;
COMMENT ON FUNCTION inventory.sp_list_batches_by_product IS
    'Lots of a product in FIFO order, each carrying the combination it belongs '
    'to. p_sku_id narrows the list to one combination; omitted, it returns every '
    'combination. p_only_active hides expired and voided lots (INV-014).';

DROP FUNCTION IF EXISTS inventory.sp_get_expiring_batches(UUID, INT, INT, INT);
CREATE FUNCTION inventory.sp_get_expiring_batches(
    p_tenant_id    UUID,
    p_warning_days INT
)
RETURNS TABLE(
    id               UUID,
    product_id       UUID,
    lot_number       VARCHAR,
    purchase_date    DATE,
    expiry_date      DATE,
    unit_cost        DECIMAL,
    initial_quantity DECIMAL,
    current_quantity DECIMAL,
    status           VARCHAR,
    days_to_expiry   INT,
    product_name     VARCHAR,
    sku_id           UUID,
    sku              VARCHAR
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(pb.status AS VARCHAR),
        CAST(pb.expiry_date - CURRENT_DATE AS INT),
        CAST(p.name AS VARCHAR),
        pb.sku_id,
        CAST(s.sku AS VARCHAR)
    FROM inventory.product_batches pb
    INNER JOIN inventory.products p
        ON p.id = pb.product_id
        AND p.tenant_id = pb.tenant_id
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.tenant_id = p_tenant_id
        AND pb.expiry_date - CURRENT_DATE <= p_warning_days
        AND pb.expiry_date - CURRENT_DATE > 0
        AND pb.status IN ('active', 'expiring_soon')
    ORDER BY pb.expiry_date ASC;
END;
$$;
COMMENT ON FUNCTION inventory.sp_get_expiring_batches IS
    'Lots expiring within the warning window, with the product name and the '
    'combination each lot belongs to (INV-014). A voided lot never appears: the '
    'status filter already excludes anything that is not active or expiring.';
