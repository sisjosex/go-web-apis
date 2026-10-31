CREATE OR REPLACE FUNCTION inventory.sp_list_counts(p_tenant_id uuid, p_status character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(row_data jsonb, chips jsonb, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    WITH chips AS (
        SELECT jsonb_build_object(
            'open', COUNT(*) FILTER (WHERE s.status IN ('open', 'paused')),
            'review', COUNT(*) FILTER (WHERE s.status = 'review'),
            'closed', COUNT(*) FILTER (WHERE s.status = 'closed')
        ) AS counts
        FROM inventory.stock_counts s
        WHERE s.tenant_id = p_tenant_id
    ), page AS (
        SELECT sc.*, COUNT(*) OVER () AS total
        FROM inventory.stock_counts sc
        WHERE sc.tenant_id = p_tenant_id
          AND (p_status IS NULL OR p_status = '' OR sc.status = ANY (string_to_array(p_status, ',')))
        ORDER BY sc.started_at DESC, sc.id
        LIMIT p_page_size OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        CASE WHEN sc.id IS NULL THEN NULL ELSE jsonb_build_object(
            'id', sc.id, 'number', sc.number, 'status', sc.status, 'blind', sc.blind,
            'adjustment_number', sc.adjustment_number, 'started_at', sc.started_at,
            'warehouse', jsonb_build_object('id', w.id, 'name', w.name),
            'category', CASE WHEN c.id IS NULL THEN NULL ELSE jsonb_build_object('id', c.id, 'name', c.name) END,
            'created_by_name', (SELECT COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) FROM auth.users u WHERE u.id = sc.created_by)
    ) || (
            SELECT jsonb_build_object(
                'total_lines', COUNT(*),
                'counted_lines', COUNT(*) FILTER (WHERE l.counted IS NOT NULL),
                'with_difference', COUNT(*) FILTER (WHERE l.counted <> l.expected)
            )
            FROM inventory.stock_count_lines l
            WHERE l.count_id = sc.id
        ) END,
        ch.counts,
        COALESCE(sc.total, 0)
    FROM chips ch
    LEFT JOIN page sc ON TRUE
    LEFT JOIN inventory.warehouses w ON w.id = sc.warehouse_id
    LEFT JOIN inventory.product_categories c ON c.id = sc.category_id
    ORDER BY sc.started_at DESC, sc.id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_counts(uuid,character varying,integer,integer) IS 'A page of counts, newest first, with the numbers behind the chips; p_status takes several, comma-separated (INV-026).';
