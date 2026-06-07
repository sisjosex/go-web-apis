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
    v_po_status VARCHAR;
    v_message TEXT := 'Purchase order approved';
BEGIN
    -- Check existence separately so callers get a distinct not-found error
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.purchase_orders po
        WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'po.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check status is draft
    SELECT po.status INTO v_po_status
    FROM purchasing.purchase_orders po
    WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id;

    IF v_po_status != 'draft' THEN
        RAISE EXCEPTION 'po.cannot-approve' USING ERRCODE = 'P0001';
    END IF;

    -- Validate PO has items
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.purchase_order_items poi
        WHERE poi.purchase_order_id = p_po_id
    ) THEN
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
    'Approves a draft purchase order. Raises po.not-found if missing, po.cannot-approve if not in draft status, po.no-items if no items exist.';
