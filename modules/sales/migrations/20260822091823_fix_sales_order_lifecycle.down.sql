-- Rollback: fix_sales_order_lifecycle
-- Module:    sales
--
-- Restores sp_create_sales_order and sp_create_return exactly as
-- 20260305000009 and 20260305000015 left them — broken timestamp expression,
-- narrowed result sets and all — and puts the original status CHECK back.
-- sales.return_seq is dropped because nothing before this migration referenced
-- it; sales.order_seq is left alone because 20260303000005 owns it and its own
-- .down.sql drops it.

DROP FUNCTION IF EXISTS sales.sp_create_sales_order(UUID, UUID, VARCHAR, TEXT, DECIMAL);
CREATE FUNCTION sales.sp_create_sales_order(
    p_tenant_id UUID,
    p_customer_id UUID,
    p_shipping_address VARCHAR,
    p_notes TEXT,
    p_discount_amount DECIMAL DEFAULT 0
)
RETURNS TABLE(
    order_id UUID,
    order_number VARCHAR,
    customer_id UUID,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    created_at TIMESTAMP,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_order_id UUID := gen_random_uuid();
    v_order_number VARCHAR;
    v_sub_total DECIMAL := 0;
    v_tax_amount DECIMAL := 0;
    v_total DECIMAL := 0;
    v_message TEXT := 'Order created successfully';
BEGIN
    -- Validate customer exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.customers c WHERE c.id = p_customer_id AND c.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Generate order number (YYYY-MM-DD-XXXXX format)
    v_order_number := TO_CHAR(CURRENT_DATE, 'YYYY-MM-DD') || '-' ||
                      LPAD(CAST(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP * 1000) AS BIGINT)::TEXT, 5, '0');

    -- Create order with pending status
    INSERT INTO sales.sales_orders(
        id, tenant_id, customer_id, order_number, shipping_address, notes,
        discount_amount, status, sub_total, tax_amount, total
    )
    VALUES(
        v_order_id, p_tenant_id, p_customer_id, v_order_number, p_shipping_address, p_notes,
        COALESCE(p_discount_amount, 0), 'pending', 0, 0, 0
    );

    RETURN QUERY
    SELECT
        v_order_id,
        v_order_number,
        p_customer_id,
        'pending'::VARCHAR,
        v_sub_total,
        v_tax_amount,
        v_total,
        COALESCE(p_discount_amount, 0),
        CURRENT_TIMESTAMP,
        v_message::TEXT;
END;
$$;

DROP FUNCTION IF EXISTS sales.sp_create_return(UUID, UUID, VARCHAR);
CREATE FUNCTION sales.sp_create_return(
    p_tenant_id UUID,
    p_order_id UUID,
    p_reason VARCHAR
)
RETURNS TABLE(
    return_id UUID,
    return_number VARCHAR,
    order_id UUID,
    status VARCHAR,
    total_amount DECIMAL,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_return_id UUID := gen_random_uuid();
    v_return_number VARCHAR;
    v_order_record RECORD;
    v_total_amount DECIMAL;
BEGIN
    -- Validate order exists, belongs to tenant, and is completed
    SELECT o.id, o.customer_id, o.total INTO v_order_record
    FROM sales.sales_orders o
    WHERE o.id = p_order_id AND o.tenant_id = p_tenant_id AND o.status = 'completed';

    IF v_order_record IS NULL THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_total_amount := v_order_record.total;

    -- Generate return number
    v_return_number := 'RET-' || TO_CHAR(CURRENT_DATE, 'YYYY-MM-DD') || '-' ||
                       LPAD(CAST(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP * 1000) AS BIGINT)::TEXT, 5, '0');

    -- Create return record (returns is a child table, accessed through order)
    INSERT INTO sales.returns(
        id, order_id, customer_id, return_number, total_amount, reason, status
    )
    VALUES(
        v_return_id, p_order_id, v_order_record.customer_id, v_return_number,
        v_total_amount, p_reason, 'pending'
    );

    RETURN QUERY
    SELECT
        v_return_id,
        v_return_number,
        p_order_id,
        'pending'::VARCHAR,
        v_total_amount,
        'Return request created'::TEXT;
END;
$$;

-- The old CHECK has no 'completed', so any order completed while this migration
-- was applied would violate it and abort the rollback. They are moved to
-- 'delivered', the nearest terminal state the old list allows, before the
-- constraint is narrowed again.
UPDATE sales.sales_orders SET status = 'delivered' WHERE status = 'completed';

ALTER TABLE sales.sales_orders
    DROP CONSTRAINT IF EXISTS sales_orders_status_check;
ALTER TABLE sales.sales_orders
    ADD CONSTRAINT sales_orders_status_check
    CHECK (status IN ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled'));

DROP SEQUENCE IF EXISTS sales.return_seq;
