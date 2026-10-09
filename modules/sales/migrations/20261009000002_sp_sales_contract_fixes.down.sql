-- Restores the sales SPs as they were before INFRA-014's contract fixes.

CREATE OR REPLACE FUNCTION sales.sp_update_customer(
    p_tenant_id UUID,
    p_customer_id UUID,
    p_name VARCHAR,
    p_phone_number VARCHAR,
    p_address VARCHAR,
    p_city VARCHAR,
    p_state VARCHAR,
    p_postal_code VARCHAR,
    p_country VARCHAR
)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    email VARCHAR,
    phone_number VARCHAR,
    address VARCHAR,
    city VARCHAR,
    state VARCHAR,
    postal_code VARCHAR,
    country VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.customers c WHERE c.id = p_customer_id AND c.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE sales.customers SET
        name = COALESCE(p_name, name),
        phone_number = COALESCE(p_phone_number, phone_number),
        address = COALESCE(p_address, address),
        city = COALESCE(p_city, city),
        state = COALESCE(p_state, state),
        postal_code = COALESCE(p_postal_code, postal_code),
        country = COALESCE(p_country, country)
    WHERE id = p_customer_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        CAST(c.id AS UUID),
        CAST(c.name AS VARCHAR),
        CAST(c.email AS VARCHAR),
        CAST(c.phone_number AS VARCHAR),
        CAST(c.address AS VARCHAR),
        CAST(c.city AS VARCHAR),
        CAST(c.state AS VARCHAR),
        CAST(c.postal_code AS VARCHAR),
        CAST(c.country AS VARCHAR),
        CAST(c.status AS VARCHAR),
        CAST(c.created_at AS TIMESTAMP),
        CAST(c.updated_at AS TIMESTAMP)
    FROM sales.customers c WHERE c.id = p_customer_id AND c.tenant_id = p_tenant_id;
END;
$$;

CREATE OR REPLACE FUNCTION sales.sp_update_sales_order(
    p_tenant_id UUID,
    p_order_id UUID,
    p_status VARCHAR,
    p_shipping_address VARCHAR,
    p_notes TEXT,
    p_discount_amount DECIMAL
)
RETURNS TABLE (
    id UUID,
    customer_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    shipping_address VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_status IS NOT NULL AND p_status NOT IN ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled') THEN
        RAISE EXCEPTION 'sales-order.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    UPDATE sales.sales_orders SET
        status = COALESCE(p_status, status),
        shipping_address = COALESCE(p_shipping_address, shipping_address),
        notes = COALESCE(p_notes, notes),
        discount_amount = COALESCE(p_discount_amount, discount_amount)
    WHERE id = p_order_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so WHERE so.id = p_order_id AND so.tenant_id = p_tenant_id;
END;
$$;

CREATE OR REPLACE FUNCTION sales.sp_cancel_sales_order(
    p_tenant_id UUID,
    p_order_id UUID
)
RETURNS TABLE(
    order_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_order_record RECORD;
BEGIN
    -- Get order details (validate tenant)
    SELECT o.id, o.order_number, o.status
    INTO v_order_record
    FROM sales.sales_orders o
    WHERE o.id = p_order_id AND o.tenant_id = p_tenant_id;

    IF v_order_record IS NULL THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Cannot cancel if already completed
    IF v_order_record.status = 'completed' THEN
        RAISE EXCEPTION 'sales-order.cannot-modify' USING ERRCODE = 'P0001';
    END IF;

    -- Delete all batch assignments (reverting the reservations)
    DELETE FROM sales.order_batch_assignments WHERE order_id = p_order_id;

    -- Update order status to cancelled
    UPDATE sales.sales_orders
    SET status = 'cancelled'
    WHERE id = p_order_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        p_order_id,
        v_order_record.order_number,
        'cancelled'::VARCHAR,
        'Order cancelled successfully'::TEXT;
END;
$$;

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
