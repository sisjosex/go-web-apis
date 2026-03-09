-- Migration: sp_purchase_order_list_operations
-- Module: purchasing
-- Description: Stored procedures for purchase order query operations

-- LIST PURCHASE ORDERS (paginated with optional status filter)
CREATE OR REPLACE FUNCTION purchasing.sp_list_purchase_orders(
    p_tenant_id UUID,
    p_status VARCHAR,
    p_page_size INT,
    p_offset INT
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    supplier_id UUID,
    po_number VARCHAR,
    status VARCHAR,
    order_date DATE,
    expected_delivery_date DATE,
    total_amount DECIMAL,
    paid_amount DECIMAL,
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF p_status IS NOT NULL THEN
        RETURN QUERY
        SELECT 
            po.id,
            po.tenant_id,
            po.supplier_id,
            po.po_number,
            po.status,
            po.order_date,
            po.expected_delivery_date,
            po.total_amount,
            po.paid_amount,
            po.notes,
            po.created_by,
            po.created_at,
            po.updated_at
        FROM purchasing.purchase_orders po
        WHERE po.tenant_id = p_tenant_id AND po.status = p_status
        ORDER BY po.created_at DESC
        LIMIT p_page_size OFFSET p_offset;
    ELSE
        RETURN QUERY
        SELECT 
            po.id,
            po.tenant_id,
            po.supplier_id,
            po.po_number,
            po.status,
            po.order_date,
            po.expected_delivery_date,
            po.total_amount,
            po.paid_amount,
            po.notes,
            po.created_by,
            po.created_at,
            po.updated_at
        FROM purchasing.purchase_orders po
        WHERE po.tenant_id = p_tenant_id
        ORDER BY po.created_at DESC
        LIMIT p_page_size OFFSET p_offset;
    END IF;
END;
$$;

-- GET PURCHASE ORDERS COUNT (for pagination)
CREATE OR REPLACE FUNCTION purchasing.sp_get_purchase_orders_count(
    p_tenant_id UUID,
    p_status VARCHAR
)
RETURNS TABLE(
    total_count INT
) LANGUAGE plpgsql AS $$
BEGIN
    IF p_status IS NOT NULL THEN
        RETURN QUERY
        SELECT CAST(COUNT(*) AS INT)
        FROM purchasing.purchase_orders
        WHERE tenant_id = p_tenant_id AND status = p_status;
    ELSE
        RETURN QUERY
        SELECT CAST(COUNT(*) AS INT)
        FROM purchasing.purchase_orders
        WHERE tenant_id = p_tenant_id;
    END IF;
END;
$$;

-- GET PENDING PAYMENTS (Outstanding invoices - Accounts Payable)
CREATE OR REPLACE FUNCTION purchasing.sp_get_pending_payments(
    p_tenant_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    supplier_id UUID,
    po_number VARCHAR,
    status VARCHAR,
    order_date DATE,
    expected_delivery_date DATE,
    total_amount DECIMAL,
    paid_amount DECIMAL,
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT DISTINCT 
        po.id,
        po.tenant_id,
        po.supplier_id,
        po.po_number,
        po.status,
        po.order_date,
        po.expected_delivery_date,
        po.total_amount,
        po.paid_amount,
        po.notes,
        po.created_by,
        po.created_at,
        po.updated_at
    FROM purchasing.purchase_orders po
    JOIN purchasing.purchase_order_invoices inv ON po.id = inv.purchase_order_id
    WHERE po.tenant_id = p_tenant_id AND inv.status != 'paid'
    ORDER BY inv.due_date ASC;
END;
$$;
