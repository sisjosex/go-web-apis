CREATE OR REPLACE FUNCTION inventory.sp_list_stock(p_tenant_id uuid, p_today date, p_warehouse_id uuid DEFAULT NULL::uuid, p_search character varying DEFAULT NULL::character varying, p_category_id uuid DEFAULT NULL::uuid, p_filter character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(product_id uuid, name character varying, sku character varying, variants_count integer, low_variants integer, by_warehouse jsonb, total numeric, reorder_level numeric, status character varying, short_warehouse_id uuid, next_lot jsonb, counts jsonb, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    WITH scope AS (
        SELECT
            p.id, p.name, p.sku, st.total, st.reorder,
            inventory.fn_stock_status(st.total, st.reorder) AS status,
            st.total > 0 AND st.last_at < NOW() - INTERVAL '90 days' AS stale,
            EXISTS (
                SELECT 1 FROM inventory.product_batches b
                WHERE b.product_id = p.id AND b.tenant_id = p_tenant_id AND b.status <> 'void' AND b.current_quantity > 0
                  AND b.expiry_date BETWEEN p_today AND p_today + 30
                  AND (p_warehouse_id IS NULL OR b.warehouse_id = p_warehouse_id)
            ) AS expiring
        FROM inventory.fn_products_in_scope(p_tenant_id, p_category_id, p_search, NULL) m
        INNER JOIN inventory.products p ON p.id = m.id AND p.status = 'active'
        CROSS JOIN LATERAL (
            SELECT COALESCE(SUM(ps.current_quantity), 0) AS total, COALESCE(SUM(ps.reorder_level), 0) AS reorder,
                   MAX(ps.last_updated_at) AS last_at
            FROM inventory.product_stock ps
            WHERE ps.product_id = p.id AND (p_warehouse_id IS NULL OR ps.warehouse_id = p_warehouse_id)
        ) st
    ), chips AS (
        SELECT jsonb_build_object(
            'all', COUNT(*),
            'low', COUNT(*) FILTER (WHERE s.status IN ('low', 'critical')),
            'out_of_stock', COUNT(*) FILTER (WHERE s.status = 'out_of_stock'),
            'expiring', COUNT(*) FILTER (WHERE s.expiring),
            'stale', COUNT(*) FILTER (WHERE s.stale)
        ) AS counts
        FROM scope s
    ), page AS (
        SELECT s.*, COUNT(*) OVER () AS total_count
        FROM scope s
        WHERE p_filter IS NULL OR p_filter = ''
           OR (p_filter = 'low' AND s.status IN ('low', 'critical'))
           OR (p_filter = 'out_of_stock' AND s.status = 'out_of_stock')
           OR (p_filter = 'expiring' AND s.expiring)
           OR (p_filter = 'stale' AND s.stale)
        ORDER BY s.name, s.id
        LIMIT p_page_size OFFSET (p_page - 1) * p_page_size
    )
    -- One row with a NULL id on an empty page, so the chips keep their counts.
    SELECT
        pg.id, CAST(pg.name AS VARCHAR), CAST(pg.sku AS VARCHAR),
        CAST(v.variants AS INT), CAST(v.low AS INT),
        COALESCE(w.by_warehouse, '[]'::jsonb), pg.total, pg.reorder, CAST(pg.status AS VARCHAR), w.short_id,
        lot.next_lot, c.counts, COALESCE(pg.total_count, 0)
    FROM chips c
    LEFT JOIN page pg ON TRUE
    LEFT JOIN LATERAL (
        SELECT COUNT(*) FILTER (WHERE NOT x.is_default) AS variants,
               COUNT(*) FILTER (WHERE NOT x.is_default AND x.quantity < x.minimum) AS low
        FROM (
            SELECT sk.id, sk.is_default, COALESCE(SUM(ps.current_quantity), 0) AS quantity, COALESCE(SUM(ps.reorder_level), 0) AS minimum
            FROM inventory.product_skus sk
            LEFT JOIN inventory.product_stock ps ON ps.sku_id = sk.id AND (p_warehouse_id IS NULL OR ps.warehouse_id = p_warehouse_id)
            WHERE sk.product_id = pg.id
            GROUP BY sk.id, sk.is_default
        ) x
    ) v ON TRUE
    LEFT JOIN LATERAL (
        SELECT jsonb_agg(jsonb_build_object('warehouse_id', q.warehouse_id, 'quantity', q.quantity)) AS by_warehouse,
               (ARRAY_AGG(q.warehouse_id ORDER BY q.quantity) FILTER (WHERE q.quantity = 0))[1] AS short_id
        FROM (
            SELECT ps.warehouse_id, SUM(ps.current_quantity) AS quantity
            FROM inventory.product_stock ps
            WHERE ps.product_id = pg.id
            GROUP BY ps.warehouse_id
        ) q
    ) w ON TRUE
    LEFT JOIN LATERAL (
        SELECT jsonb_build_object('lot_number', b.lot_number, 'expiry_date', b.expiry_date) AS next_lot
        FROM inventory.product_batches b
        WHERE b.product_id = pg.id AND b.tenant_id = p_tenant_id AND b.status <> 'void' AND b.current_quantity > 0
          AND b.expiry_date >= p_today AND (p_warehouse_id IS NULL OR b.warehouse_id = p_warehouse_id)
        ORDER BY b.expiry_date
        LIMIT 1
    ) lot ON TRUE
    ORDER BY pg.name, pg.id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_stock(uuid,date,uuid,character varying,uuid,character varying,integer,integer) IS 'Stock across products and warehouses, a page with the counts behind its chips (INV-025).';
