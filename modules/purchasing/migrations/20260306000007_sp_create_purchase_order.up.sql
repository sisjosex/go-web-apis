-- Create Purchase Order stored procedure
CREATE OR REPLACE FUNCTION purchasing.sp_create_purchase_order(
    p_tenant_id UUID,
    p_supplier_id UUID,
    p_expected_delivery_date DATE,
    p_notes TEXT DEFAULT NULL,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE(
    po_id UUID,
    po_number VARCHAR,
    supplier_id UUID,
    status VARCHAR,
    total_amount DECIMAL,
    expected_delivery_date DATE,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_po_id UUID := gen_random_uuid();
    v_po_number VARCHAR;
    v_sequence INT;
    v_message TEXT := 'Purchase order created successfully';
BEGIN
    -- Validate supplier exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.suppliers s WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'supplier.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Generate PO number (PO-YYYY-00001), scoped to tenant
    v_sequence := COALESCE((SELECT COUNT(*) + 1 FROM purchasing.purchase_orders po WHERE po.tenant_id = p_tenant_id), 1);
    v_po_number := 'PO-' || TO_CHAR(CURRENT_DATE, 'YYYY') || '-' || LPAD(v_sequence::TEXT, 5, '0');

    -- Create PO with draft status
    INSERT INTO purchasing.purchase_orders(
        id, tenant_id, supplier_id, po_number, status,
        expected_delivery_date, notes, created_by
    )
    VALUES(
        v_po_id, p_tenant_id, p_supplier_id, v_po_number, 'draft',
        p_expected_delivery_date, p_notes, p_created_by
    );

    RETURN QUERY
    SELECT
        v_po_id,
        v_po_number,
        p_supplier_id,
        'draft'::VARCHAR,
        0::DECIMAL,
        p_expected_delivery_date,
        v_message::TEXT;
END;
$$;
