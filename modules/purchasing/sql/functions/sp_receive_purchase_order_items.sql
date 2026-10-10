CREATE OR REPLACE FUNCTION purchasing.sp_receive_purchase_order_items(p_tenant_id uuid, p_po_id uuid, p_receipt_date timestamp without time zone DEFAULT CURRENT_TIMESTAMP, p_received_by uuid DEFAULT NULL::uuid, p_notes text DEFAULT NULL::text)
 RETURNS TABLE(receipt_id uuid, po_id uuid, receipt_number character varying, received_by uuid, notes text, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_receipt_id UUID := gen_random_uuid();
    v_receipt_number VARCHAR;
    v_item RECORD;
BEGIN
    -- FOR UPDATE: two receipts of one order cannot both add its stock.
    PERFORM 1 FROM purchasing.purchase_orders po
    WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status IN ('approved', 'invoiced')
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'po.cannot-receive' USING ERRCODE = 'P0001';
    END IF;

    v_receipt_number := 'RCP-' || TO_CHAR(p_receipt_date, 'YYYYMMDD') || '-' || UPPER(LEFT(REPLACE(v_receipt_id::TEXT, '-', ''), 8));

    INSERT INTO purchasing.purchase_order_receipts(id, purchase_order_id, receipt_number, receipt_date, received_by, notes)
    VALUES (v_receipt_id, p_po_id, v_receipt_number, p_receipt_date, p_received_by, p_notes);

    FOR v_item IN
        SELECT poi.product_id, poi.quantity - poi.received_quantity AS pending, poi.unit_cost
        FROM purchasing.purchase_order_items poi
        WHERE poi.purchase_order_id = p_po_id AND poi.status != 'received'
    LOOP
        PERFORM inventory.sp_record_movement(
            p_tenant_id      := p_tenant_id,
            p_product_id     := v_item.product_id,
            p_movement_type  := 'PURCHASE',
            p_quantity       := v_item.pending::DECIMAL,
            p_reference_type := 'purchase_order',
            p_reference_id   := p_po_id,
            p_unit_cost      := v_item.unit_cost,
            p_notes          := v_receipt_number,
            p_created_by     := p_received_by
        );
    END LOOP;

    UPDATE purchasing.purchase_order_items poi
    SET received_quantity = poi.quantity,
        status = 'received',
        updated_at = CURRENT_TIMESTAMP
    WHERE poi.purchase_order_id = p_po_id AND poi.status != 'received';

    UPDATE purchasing.purchase_orders
    SET status = 'received',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT r.id, r.purchase_order_id, r.receipt_number, r.received_by, r.notes, r.created_at
    FROM purchasing.purchase_order_receipts r
    WHERE r.id = v_receipt_id;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_receive_purchase_order_items(uuid,uuid,timestamp without time zone,uuid,text) IS 'Receives an approved or invoiced order: one PURCHASE inventory movement per pending item, items and order marked received, the receipt row answered. Raises po.cannot-receive otherwise.';
