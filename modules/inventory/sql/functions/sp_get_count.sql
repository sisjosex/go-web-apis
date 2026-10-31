CREATE OR REPLACE FUNCTION inventory.sp_get_count(p_tenant_id uuid, p_count_id uuid)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_out JSONB;
BEGIN
    WITH lines AS (
        -- Until a line is counted, what it reads is today's stock (D1).
        SELECT l.sku_id, p.name AS product_name, s.sku,
               COALESCE(l.expected, inventory.fn_warehouse_quantity(l.sku_id, (
                   SELECT sc.warehouse_id FROM inventory.stock_counts sc WHERE sc.id = p_count_id
               ))) AS expected,
               l.counted,
               l.counted - l.expected AS difference,
               inventory.fn_sku_unit_cost(p_tenant_id, l.sku_id) AS unit_cost,
               (SELECT string_agg(vo.option_name, ' · ' ORDER BY vo.option_name)
                FROM inventory.product_sku_options so
                INNER JOIN inventory.product_variant_options vo ON vo.id = so.option_id
                WHERE so.sku_id = l.sku_id) AS label
        FROM inventory.stock_count_lines l
        INNER JOIN inventory.products p ON p.id = l.product_id
        INNER JOIN inventory.product_skus s ON s.id = l.sku_id
        WHERE l.count_id = p_count_id
    )
    SELECT jsonb_build_object(
        'id', sc.id, 'number', sc.number, 'status', sc.status, 'blind', sc.blind,
        'adjustment_number', sc.adjustment_number, 'started_at', sc.started_at,
        'warehouse', jsonb_build_object('id', w.id, 'name', w.name),
        'category', CASE WHEN c.id IS NULL THEN NULL ELSE jsonb_build_object('id', c.id, 'name', c.name) END,
        'created_by_name', (SELECT COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) FROM auth.users u WHERE u.id = sc.created_by)
    ) || jsonb_build_object(
        'lines', (
            SELECT COALESCE(jsonb_agg(jsonb_build_object(
                'sku_id', x.sku_id, 'product_name', x.product_name, 'sku', x.sku, 'label', x.label,
                'expected', x.expected, 'counted', x.counted, 'difference', x.difference, 'unit_cost', x.unit_cost
            ) ORDER BY x.product_name, x.sku), '[]'::jsonb)
            FROM lines x
        ),
        'progress', (
            SELECT jsonb_build_object(
                'counted', COUNT(*) FILTER (WHERE x.counted IS NOT NULL),
                'total', COUNT(*),
                'with_difference', COUNT(*) FILTER (WHERE x.difference <> 0),
                -- At cost, and what has no cost does not add (D2).
                'difference_value', ROUND(COALESCE(SUM(x.difference * x.unit_cost), 0), 2)
            )
            FROM lines x
        )
    )
    INTO v_out
    FROM inventory.stock_counts sc
    INNER JOIN inventory.warehouses w ON w.id = sc.warehouse_id
    LEFT JOIN inventory.product_categories c ON c.id = sc.category_id
    WHERE sc.id = p_count_id AND sc.tenant_id = p_tenant_id;

    IF v_out IS NULL THEN
        RAISE EXCEPTION 'inventory.count.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_out;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_count(uuid,uuid) IS 'A count with its lines and progress, the difference at cost (INV-026 D2). The controller hides what was expected from a blind counter. Raises inventory.count.not-found.';
