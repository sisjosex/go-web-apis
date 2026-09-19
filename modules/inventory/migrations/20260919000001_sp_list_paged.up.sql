-- APP-004 step 3: the three inventory lists the app pages move to the purchasing shape —
-- p_page / p_page_size in, total_count out — so the endpoints answer
-- { <plural>, total_count, page, page_size } and the app pages server-side.
--
-- Signatures and return types change: DROP + CREATE (sql.md). p_page_size is capped by the
-- controller's binding (max 100); the SPs trust it.
--
-- Cost, per list:
--   products  the page is chosen first on bare columns (CTE) and only its rows build the
--             categories JSON; a single query with COUNT(*) OVER () would build it for every
--             row the filters match before LIMIT threw them away.
--             idx_products_tenant_active_created serves filter + order; the trigram indexes
--             serve the '%term%' search on name and sku.
--   batches   (product_id, expiry_date) already serves it; the lot count per product is small.
--   expiring  the window predicate is rewritten from `expiry_date - CURRENT_DATE <= n`, which no
--             index can use, to a range on expiry_date, served by a partial index on open lots.

-- ===========================================================================
-- Indexes
-- ===========================================================================
CREATE INDEX IF NOT EXISTS idx_products_tenant_active_created
    ON inventory.products (tenant_id, created_at DESC, id DESC)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_products_name_trgm
    ON inventory.products USING GIN (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_products_sku_trgm
    ON inventory.products USING GIN (sku gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_product_batches_tenant_open_expiry
    ON inventory.product_batches (tenant_id, expiry_date)
    WHERE status IN ('active', 'expiring_soon');

-- ===========================================================================
-- sp_list_products
-- ===========================================================================
DROP FUNCTION IF EXISTS inventory.sp_list_products(UUID, INT, INT, UUID, VARCHAR);
CREATE FUNCTION inventory.sp_list_products(
    p_tenant_id   UUID,
    p_page        INT     DEFAULT 1,
    p_page_size   INT     DEFAULT 20,
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
    categories   JSONB,
    total_count  BIGINT
) LANGUAGE plpgsql AS $$
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
$$;

COMMENT ON FUNCTION inventory.sp_list_products(UUID, INT, INT, UUID, VARCHAR) IS
'One page of a tenant''s active products, newest first, optionally narrowed to a category and a name/SKU search; each row carries its categories and the total_count the filters match';

-- ===========================================================================
-- sp_list_batches_by_product
-- ===========================================================================
DROP FUNCTION IF EXISTS inventory.sp_list_batches_by_product(UUID, UUID, BOOLEAN, UUID);
CREATE FUNCTION inventory.sp_list_batches_by_product(
    p_tenant_id   UUID,
    p_product_id  UUID,
    p_only_active BOOLEAN DEFAULT TRUE,
    p_sku_id      UUID    DEFAULT NULL,
    p_page        INT     DEFAULT 1,
    p_page_size   INT     DEFAULT 20
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
    sku              VARCHAR,
    total_count      BIGINT
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
        CAST(s.sku AS VARCHAR),
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM inventory.product_batches pb
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.tenant_id = p_tenant_id
        AND pb.product_id = p_product_id
        AND (p_sku_id IS NULL OR pb.sku_id = p_sku_id)
        AND (NOT p_only_active OR pb.status NOT IN ('expired', 'void'))
    -- FIFO order; pb.id breaks ties so a page boundary never splits a tie differently twice.
    ORDER BY pb.expiry_date ASC, pb.created_at ASC, pb.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$;

COMMENT ON FUNCTION inventory.sp_list_batches_by_product(UUID, UUID, BOOLEAN, UUID, INT, INT) IS
'One page of a product''s lots in FIFO order, each with its combination and the total_count the filters match. p_sku_id narrows to one combination; p_only_active hides expired and voided lots';

-- ===========================================================================
-- sp_get_expiring_batches
-- ===========================================================================
DROP FUNCTION IF EXISTS inventory.sp_get_expiring_batches(UUID, INT);
CREATE FUNCTION inventory.sp_get_expiring_batches(
    p_tenant_id    UUID,
    p_warning_days INT,
    p_page         INT DEFAULT 1,
    p_page_size    INT DEFAULT 20
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
    sku              VARCHAR,
    total_count      BIGINT
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
        CAST(s.sku AS VARCHAR),
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM inventory.product_batches pb
    INNER JOIN inventory.products p
        ON p.id = pb.product_id
        AND p.tenant_id = pb.tenant_id
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.tenant_id = p_tenant_id
        -- Same window as before (1..n days out, never what already expired), as a range
        -- on the column so idx_product_batches_tenant_open_expiry can serve it.
        AND pb.expiry_date >  CURRENT_DATE
        AND pb.expiry_date <= CURRENT_DATE + p_warning_days
        AND pb.status IN ('active', 'expiring_soon')
    ORDER BY pb.expiry_date ASC, pb.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$;

COMMENT ON FUNCTION inventory.sp_get_expiring_batches(UUID, INT, INT, INT) IS
'One page of the lots expiring within the warning window, soonest first, with the product name, the combination and the total_count. A voided lot never appears: the status filter excludes anything not active or expiring';
