CREATE OR REPLACE FUNCTION inventory.sp_get_transfer(p_tenant_id uuid, p_transfer_id uuid)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_out JSONB;
BEGIN
    SELECT jsonb_build_object(
        'id', t.id, 'number', t.number, 'status', t.status, 'notes', t.notes,
        'from', jsonb_build_object('id', wf.id, 'name', wf.name),
        'to', jsonb_build_object('id', wt.id, 'name', wt.name),
        'lines', (
            SELECT COALESCE(jsonb_agg(jsonb_build_object(
                'sku_id', l.sku_id, 'product_id', l.product_id,
                'label', p.name || COALESCE(' · ' || o.options, ''),
                'quantity', l.quantity, 'received_quantity', l.received_quantity,
                'lots', (
                    SELECT COALESCE(jsonb_agg(jsonb_build_object(
                        'lot_number', s.lot_number, 'expiry_date', s.expiry_date, 'quantity', b.quantity
                    ) ORDER BY b.position), '[]'::jsonb)
                    FROM inventory.transfer_line_batches b
                    INNER JOIN inventory.product_batches s ON s.id = b.source_batch_id
                    WHERE b.transfer_line_id = l.id
                )
            ) ORDER BY p.name), '[]'::jsonb)
            FROM inventory.transfer_lines l
            INNER JOIN inventory.products p ON p.id = l.product_id
            LEFT JOIN LATERAL (
                SELECT string_agg(vo.option_name, ' · ') AS options
                FROM inventory.product_sku_options so
                INNER JOIN inventory.product_variant_options vo ON vo.id = so.option_id
                WHERE so.sku_id = l.sku_id
            ) o ON TRUE
            WHERE l.transfer_id = t.id
        ),
        'units', (SELECT COALESCE(SUM(l.quantity), 0) FROM inventory.transfer_lines l WHERE l.transfer_id = t.id),
        'products', (SELECT COUNT(DISTINCT l.product_id) FROM inventory.transfer_lines l WHERE l.transfer_id = t.id),
        'timeline', (
            SELECT jsonb_agg(step ORDER BY (step ->> 'at'))
            FROM (
                SELECT jsonb_build_object('step', 'prepared', 'at', t.prepared_at, 'by_name', (SELECT CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR) FROM auth.users u WHERE u.id = t.prepared_by), 'note', t.notes) AS step
                UNION ALL
                SELECT jsonb_build_object('step', 'in_transit', 'at', t.shipped_at, 'by_name', (SELECT CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR) FROM auth.users u WHERE u.id = t.shipped_by), 'note', t.carrier_note)
                WHERE t.shipped_at IS NOT NULL
                UNION ALL
                SELECT jsonb_build_object('step', 'received', 'at', t.received_at, 'by_name', (SELECT CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR) FROM auth.users u WHERE u.id = t.received_by), 'note', NULL)
                WHERE t.received_at IS NOT NULL
            ) steps
        )
    )
    INTO v_out
    FROM inventory.transfers t
    INNER JOIN inventory.warehouses wf ON wf.id = t.from_warehouse_id
    INNER JOIN inventory.warehouses wt ON wt.id = t.to_warehouse_id
    WHERE t.id = p_transfer_id AND t.tenant_id = p_tenant_id;

    IF v_out IS NULL THEN
        RAISE EXCEPTION 'inventory.transfer.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_out;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_transfer(uuid,uuid) IS 'A transfer with its lines and timeline. Raises inventory.transfer.not-found.';
