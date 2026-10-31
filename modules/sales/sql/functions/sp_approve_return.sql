CREATE OR REPLACE FUNCTION sales.sp_approve_return(p_tenant_id uuid, p_return_id uuid)
 RETURNS TABLE(return_id uuid, return_number character varying, status character varying, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_return_record RECORD;
    v_order_id UUID;
    v_batch_record RECORD;
BEGIN
    -- Get return details and validate through order's tenant
    SELECT r.id, r.return_number, r.order_id, r.status
    INTO v_return_record
    FROM sales.returns r
    JOIN sales.sales_orders o ON o.id = r.order_id
    WHERE r.id = p_return_id AND o.tenant_id = p_tenant_id;

    IF v_return_record IS NULL THEN
        RAISE EXCEPTION 'return.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_return_record.status != 'pending' THEN
        RAISE EXCEPTION 'return.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    v_order_id := v_return_record.order_id;

    -- Restore batch quantities from the order assignments
    FOR v_batch_record IN
        SELECT oba.product_batch_id, oba.quantity_assigned, pb.product_id, pb.sku_id, pb.warehouse_id
        FROM sales.order_batch_assignments oba
        INNER JOIN inventory.product_batches pb ON pb.id = oba.product_batch_id
        WHERE oba.order_id = v_order_id
    LOOP
        -- INV-027: the units return to their lot and to the stock of its warehouse, by the one door.
        PERFORM inventory.sp_record_movement(
            p_tenant_id := p_tenant_id, p_product_id := v_batch_record.product_id, p_movement_type := 'RETURN',
            p_quantity := v_batch_record.quantity_assigned, p_reference_type := 'RETURN', p_reference_id := p_return_id,
            p_sku_id := v_batch_record.sku_id, p_reference_label := 'Devolución #' || v_return_record.return_number,
            p_warehouse_id := v_batch_record.warehouse_id, p_batch_id := v_batch_record.product_batch_id
        );
    END LOOP;

    -- Update return status to approved
    UPDATE sales.returns
    SET status = 'approved'
    WHERE id = p_return_id;

    RETURN QUERY
    SELECT
        p_return_id,
        v_return_record.return_number,
        'approved'::VARCHAR,
        'Return approved and inventory restored'::TEXT;
END;
$function$;

COMMENT ON FUNCTION sales.sp_approve_return(uuid,uuid) IS 'Approves a return: its units go back to their lot and to the stock of its warehouse (INV-027). Raises return.not-found, return.invalid-status.';
