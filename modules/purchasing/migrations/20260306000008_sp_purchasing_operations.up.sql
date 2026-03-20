-- Add item to purchase order
CREATE OR REPLACE FUNCTION purchasing.sp_add_purchase_order_item(
    p_tenant_id UUID,
    p_po_id UUID,
    p_product_id UUID,
    p_quantity INT,
    p_unit_cost DECIMAL
)
RETURNS TABLE(
    item_id UUID,
    po_id UUID,
    product_id UUID,
    quantity INT,
    unit_cost DECIMAL,
    line_total DECIMAL,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_item_id UUID := gen_random_uuid();
    v_line_total DECIMAL;
    v_message TEXT := 'Item added to purchase order';
BEGIN
    -- Validate PO exists, belongs to tenant, and is in draft/approved status
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_orders po WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status IN ('draft', 'approved')) THEN
        RAISE EXCEPTION 'po.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    -- Validate product exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate quantity and cost
    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'po.invalid-quantity' USING ERRCODE = 'P0001';
    END IF;

    IF p_unit_cost <= 0 THEN
        RAISE EXCEPTION 'po.invalid-cost' USING ERRCODE = 'P0001';
    END IF;

    v_line_total := p_quantity * p_unit_cost;

    -- Insert item
    INSERT INTO purchasing.purchase_order_items(
        id, purchase_order_id, product_id, quantity, unit_cost
    )
    VALUES(v_item_id, p_po_id, p_product_id, p_quantity, p_unit_cost);

    -- Update PO total
    UPDATE purchasing.purchase_orders
    SET total_amount = (
        SELECT COALESCE(SUM(poi.line_total), 0)
        FROM purchasing.purchase_order_items poi
        WHERE poi.purchase_order_id = p_po_id
    ),
    updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        v_item_id,
        p_po_id,
        p_product_id,
        p_quantity,
        p_unit_cost,
        v_line_total,
        v_message::TEXT;
END;
$$;

-- Approve purchase order
CREATE OR REPLACE FUNCTION purchasing.sp_approve_purchase_order(
    p_tenant_id UUID,
    p_po_id UUID
)
RETURNS TABLE(
    po_id UUID,
    status VARCHAR,
    total_amount DECIMAL,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_message TEXT := 'Purchase order approved';
BEGIN
    -- Validate PO exists, belongs to tenant, and is in draft
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_orders po WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status = 'draft') THEN
        RAISE EXCEPTION 'po.cannot-approve' USING ERRCODE = 'P0001';
    END IF;

    -- Validate PO has items
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_order_items poi WHERE poi.purchase_order_id = p_po_id) THEN
        RAISE EXCEPTION 'po.no-items' USING ERRCODE = 'P0001';
    END IF;

    -- Update status to approved
    UPDATE purchasing.purchase_orders
    SET status = 'approved',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        p_po_id,
        'approved'::VARCHAR,
        (SELECT po.total_amount FROM purchasing.purchase_orders po WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id),
        v_message::TEXT;
END;
$$;

-- Receive purchase order items
CREATE OR REPLACE FUNCTION purchasing.sp_receive_purchase_order_items(
    p_tenant_id UUID,
    p_po_id UUID,
    p_receipt_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    p_received_by UUID DEFAULT NULL,
    p_notes TEXT DEFAULT NULL
)
RETURNS TABLE(
    receipt_id UUID,
    po_id UUID,
    po_status VARCHAR,
    items_received INT,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_receipt_id UUID := gen_random_uuid();
    v_receipt_number VARCHAR;
    v_items_count INT;
    v_message TEXT := 'Goods received and inventory updated';
BEGIN
    -- Validate PO exists, belongs to tenant, and is approved or invoiced
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_orders po WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status IN ('approved', 'invoiced')) THEN
        RAISE EXCEPTION 'po.cannot-receive' USING ERRCODE = 'P0001';
    END IF;

    -- Generate receipt number
    v_receipt_number := 'RCP-' || TO_CHAR(CURRENT_DATE, 'YYYYMMDD') || '-' || LPAD(CAST(EXTRACT(EPOCH FROM p_receipt_date) * 1000 AS BIGINT)::TEXT, 5, '0');

    -- Create receipt
    INSERT INTO purchasing.purchase_order_receipts(
        id, purchase_order_id, receipt_number, receipt_date, received_by, notes
    )
    VALUES(v_receipt_id, p_po_id, v_receipt_number, p_receipt_date, p_received_by, p_notes);

    -- Mark all items as received
    UPDATE purchasing.purchase_order_items poi
    SET received_quantity = poi.quantity,
        status = 'received',
        updated_at = CURRENT_TIMESTAMP
    WHERE poi.purchase_order_id = p_po_id AND poi.status != 'received';

    GET DIAGNOSTICS v_items_count = ROW_COUNT;

    -- Update PO status to received
    UPDATE purchasing.purchase_orders
    SET status = 'received',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        v_receipt_id,
        p_po_id,
        'received'::VARCHAR,
        v_items_count,
        v_message::TEXT;
END;
$$;
