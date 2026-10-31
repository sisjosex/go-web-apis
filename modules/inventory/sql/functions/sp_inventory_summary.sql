CREATE OR REPLACE FUNCTION inventory.sp_inventory_summary(p_tenant_id uuid, p_today date, p_warehouse_id uuid DEFAULT NULL::uuid)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_out JSONB;
BEGIN
    WITH rows AS (
        SELECT ps.product_id, ps.sku_id, ps.warehouse_id, ps.current_quantity AS quantity, ps.reorder_level AS minimum,
               p.name, p.sku, s.is_default
        FROM inventory.product_stock ps
        INNER JOIN inventory.products p ON p.id = ps.product_id AND p.tenant_id = p_tenant_id AND p.status = 'active'
        INNER JOIN inventory.product_skus s ON s.id = ps.sku_id
        WHERE p_warehouse_id IS NULL OR ps.warehouse_id = p_warehouse_id
    ), per_product AS (
        SELECT r.product_id, inventory.fn_stock_status(SUM(r.quantity), SUM(r.minimum)) AS status
        FROM rows r
        GROUP BY r.product_id
    ), short AS (
        SELECT r.*, w.name AS warehouse_name
        FROM rows r
        INNER JOIN inventory.warehouses w ON w.id = r.warehouse_id
        WHERE r.quantity < r.minimum
    ), urgent AS (
        -- Out of stock first, then the smallest share of the minimum.
        SELECT sh.*
        FROM short sh
        ORDER BY (sh.quantity > 0), sh.quantity / NULLIF(sh.minimum, 0), sh.name
        LIMIT 4
    ), reorder AS (
        SELECT u.*, sold.weekly, origin.warehouse_id AS from_id, origin.available AS from_quantity
        FROM urgent u
        LEFT JOIN LATERAL (
            SELECT COALESCE(SUM(-m.quantity), 0) / 4 AS weekly
            FROM inventory.inventory_movements m
            WHERE m.tenant_id = p_tenant_id AND m.sku_id = u.sku_id AND m.warehouse_id = u.warehouse_id
              AND m.movement_type = 'SALE' AND m.created_at >= p_today - 28
        ) sold ON TRUE
        LEFT JOIN LATERAL (
            -- What another warehouse holds beyond its own minimum can come over instead of a purchase.
            SELECT o.warehouse_id, o.current_quantity - o.reserved_quantity AS available
            FROM inventory.product_stock o
            WHERE o.sku_id = u.sku_id AND o.warehouse_id <> u.warehouse_id
              AND o.current_quantity - o.reserved_quantity > o.reorder_level
            ORDER BY o.current_quantity - o.reserved_quantity DESC
            LIMIT 1
        ) origin ON TRUE
    )
    SELECT jsonb_build_object(
        'warehouses_count', (SELECT COUNT(*) FROM inventory.warehouses w WHERE w.tenant_id = p_tenant_id AND w.status = 'active'),
        'value', (SELECT COALESCE(ROUND(SUM(r.quantity * inventory.fn_sku_unit_cost(p_tenant_id, r.sku_id)), 2), 0) FROM rows r WHERE r.quantity > 0),
        'products', (SELECT COUNT(DISTINCT r.product_id) FROM rows r),
        'variants', (SELECT COUNT(DISTINCT r.sku_id) FROM rows r WHERE NOT r.is_default),
        'low', (SELECT COUNT(*) FROM per_product x WHERE x.status IN ('low', 'critical')),
        'out_of_stock', (SELECT COUNT(*) FROM per_product x WHERE x.status = 'out_of_stock'),
        'expiring', (
            SELECT COUNT(DISTINCT b.product_id) FROM inventory.product_batches b
            WHERE b.tenant_id = p_tenant_id AND b.status <> 'void' AND b.current_quantity > 0
              AND b.expiry_date BETWEEN p_today AND p_today + 30
        ),
        'reorder', jsonb_build_object(
            'total', (SELECT COUNT(*) FROM short),
            'items', (
                SELECT COALESCE(jsonb_agg(jsonb_build_object(
                    'product_id', r.product_id, 'sku_id', r.sku_id, 'name', r.name, 'sku', r.sku,
                    'warehouse_id', r.warehouse_id, 'warehouse_name', r.warehouse_name,
                    'quantity', r.quantity, 'reorder_level', r.minimum,
                    'suggested', GREATEST(CEIL(r.weekly * 2 - r.quantity), CEIL(r.minimum - r.quantity)),
                    'transfer_from', CASE WHEN r.from_id IS NULL THEN NULL
                                          ELSE jsonb_build_object('warehouse_id', r.from_id, 'quantity', r.from_quantity) END
                ) ORDER BY (r.quantity > 0), r.quantity / NULLIF(r.minimum, 0), r.name), '[]'::jsonb)
                FROM reorder r
            )
        ),
        'top_sellers', (
            SELECT COALESCE(jsonb_agg(jsonb_build_object('product_id', t.product_id, 'name', t.name, 'quantity', t.quantity) ORDER BY t.quantity DESC), '[]'::jsonb)
            FROM (
                SELECT m.product_id, p.name, SUM(-m.quantity) AS quantity
                FROM inventory.inventory_movements m
                INNER JOIN inventory.products p ON p.id = m.product_id
                WHERE m.tenant_id = p_tenant_id AND m.movement_type = 'SALE' AND m.created_at >= p_today - 7
                  AND (p_warehouse_id IS NULL OR m.warehouse_id = p_warehouse_id)
                GROUP BY m.product_id, p.name
                ORDER BY quantity DESC
                LIMIT 4
            ) t
        )
    ) INTO v_out;
    RETURN v_out;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_inventory_summary(uuid,date,uuid) IS 'The inventory home in one query: value at cost, counts, what to restock with its suggestion (two weeks of the last four weeks of sales, at least up to the minimum) and the week''s best sellers (INV-025 D4).';
