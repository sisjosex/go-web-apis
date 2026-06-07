DROP FUNCTION IF EXISTS purchasing.sp_approve_purchase_order(UUID, UUID);

CREATE FUNCTION purchasing.sp_approve_purchase_order(
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

COMMENT ON FUNCTION purchasing.sp_approve_purchase_order(UUID, UUID) IS
    'Approves a draft purchase order. Raises po.cannot-approve if not found or not in draft status.';
