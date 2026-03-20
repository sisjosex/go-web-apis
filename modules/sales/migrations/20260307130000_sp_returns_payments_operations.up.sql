-- Returns & Payments Operations Stored Procedures
-- Phase: Sales Module Compliance (Direct Queries → Stored Procedures)

-- ===== RETURNS OPERATIONS =====

-- sp_get_return: Get a return by ID
CREATE OR REPLACE FUNCTION sales.sp_get_return(
    p_tenant_id UUID,
    p_return_id UUID
)
RETURNS TABLE (
    id UUID,
    order_id UUID,
    customer_id UUID,
    return_number VARCHAR,
    total_amount DECIMAL,
    reason TEXT,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        CAST(r.id AS UUID),
        CAST(r.order_id AS UUID),
        CAST(r.customer_id AS UUID),
        CAST(r.return_number AS VARCHAR),
        r.total_amount,
        r.reason,
        CAST(r.status AS VARCHAR),
        r.created_at,
        r.updated_at
    FROM sales.returns r
    JOIN sales.sales_orders o ON o.id = r.order_id
    WHERE r.id = p_return_id AND o.tenant_id = p_tenant_id;
END;
$$;

-- sp_get_returns_by_order: Get all returns for an order
CREATE OR REPLACE FUNCTION sales.sp_get_returns_by_order(
    p_tenant_id UUID,
    p_order_id UUID
)
RETURNS TABLE (
    id UUID,
    order_id UUID,
    customer_id UUID,
    return_number VARCHAR,
    total_amount DECIMAL,
    reason TEXT,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    -- Validate order belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(r.id AS UUID),
        CAST(r.order_id AS UUID),
        CAST(r.customer_id AS UUID),
        CAST(r.return_number AS VARCHAR),
        r.total_amount,
        r.reason,
        CAST(r.status AS VARCHAR),
        r.created_at,
        r.updated_at
    FROM sales.returns r
    WHERE r.order_id = p_order_id
    ORDER BY r.created_at DESC;
END;
$$;

-- ===== PAYMENTS OPERATIONS =====

-- sp_get_payments: Get all payments for an order
CREATE OR REPLACE FUNCTION sales.sp_get_payments(
    p_tenant_id UUID,
    p_order_id UUID
)
RETURNS TABLE (
    id UUID,
    order_id UUID,
    customer_id UUID,
    amount DECIMAL,
    payment_method VARCHAR,
    status VARCHAR,
    reference_number VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    -- Validate order belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(p.id AS UUID),
        CAST(p.order_id AS UUID),
        CAST(p.customer_id AS UUID),
        p.amount,
        CAST(p.payment_method AS VARCHAR),
        CAST(p.status AS VARCHAR),
        CAST(p.reference_number AS VARCHAR),
        p.notes,
        p.created_at,
        p.updated_at
    FROM sales.payments p
    WHERE p.order_id = p_order_id
    ORDER BY p.created_at DESC;
END;
$$;

-- sp_get_payment_by_id: Get a payment by ID
CREATE OR REPLACE FUNCTION sales.sp_get_payment_by_id(
    p_tenant_id UUID,
    p_payment_id UUID
)
RETURNS TABLE (
    id UUID,
    order_id UUID,
    customer_id UUID,
    amount DECIMAL,
    payment_method VARCHAR,
    status VARCHAR,
    reference_number VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        CAST(p.id AS UUID),
        CAST(p.order_id AS UUID),
        CAST(p.customer_id AS UUID),
        p.amount,
        CAST(p.payment_method AS VARCHAR),
        CAST(p.status AS VARCHAR),
        CAST(p.reference_number AS VARCHAR),
        p.notes,
        p.created_at,
        p.updated_at
    FROM sales.payments p
    JOIN sales.sales_orders o ON o.id = p.order_id
    WHERE p.id = p_payment_id AND o.tenant_id = p_tenant_id;
END;
$$;
