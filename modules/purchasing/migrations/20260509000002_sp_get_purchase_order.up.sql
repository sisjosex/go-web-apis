-- Migration: sp_get_purchase_order
-- Module: purchasing
-- Description: Stored procedure to retrieve a single purchase order by ID

CREATE OR REPLACE FUNCTION purchasing.sp_get_purchase_order(
    p_po_id     UUID,
    p_tenant_id UUID
)
RETURNS TABLE(
    id                     UUID,
    supplier_id            UUID,
    po_number              VARCHAR,
    status                 VARCHAR,
    order_date             DATE,
    expected_delivery_date DATE,
    total_amount           DECIMAL,
    paid_amount            DECIMAL,
    notes                  TEXT,
    created_by             UUID,
    created_at             TIMESTAMP,
    updated_at             TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.purchase_orders po
        WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'po.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        po.id,
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
    WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id;
END;
$$;

COMMENT ON FUNCTION purchasing.sp_get_purchase_order(UUID, UUID) IS
    'Retrieves a single purchase order by ID scoped to a tenant. Raises po.not-found if not found.';
