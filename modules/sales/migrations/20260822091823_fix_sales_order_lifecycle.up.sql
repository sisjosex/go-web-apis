-- Migration: fix_sales_order_lifecycle
-- Module:    sales
-- Created:   2026-08-22 09:18:23
--
-- POST /sales/orders has been returning 500 since the v2 rewrite
-- (20260305000009). Four independent defects stack up on the same path; the
-- module's tests hid all of them behind t.Skipf on a non-201, so the suite
-- stayed green while the whole order lifecycle was dead.
--
--   1. Document numbers were built with
--          LPAD(CAST(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP * 1000) AS BIGINT)::TEXT, 5, '0')
--      The closing parenthesis sits after `* 1000`, so PostgreSQL is asked for
--      `timestamptz * integer` — SQLSTATE 42883 on every call.
--   2. Even with the parenthesis moved, the suffix is unusable: epoch-in-ms is
--      13 digits and LPAD *truncates* when the value is longer than the width,
--      so every order of the same era collapses onto the same 5 leading digits
--      and collides on the (tenant_id, order_number) unique index. Both SPs go
--      back to a sequence, which is what sp_create_sales_order used before the
--      v2 rewrite. LPAD gets GREATEST(5, LENGTH(...)) as its width so it pads
--      to five digits and never truncates once the sequence passes 99999.
--   3. sp_create_sales_order returned a 10-column result (order_id,
--      order_number, customer_id, …, created_at, message) with created_at
--      declared TIMESTAMP but populated from CURRENT_TIMESTAMP (timestamptz),
--      while SalesOrderRepository.CreateSalesOrder scans the canonical
--      12-column order row. sp_create_return had the same problem against its
--      own caller, which selects the 9 columns of a sales.returns row by name.
--      Both now return the shape their sibling SPs already return
--      (sp_get_sales_order_by_id, sp_get_returns_by_order) and Go already
--      scans, with the timestamps read from the stored row.
--   4. sales_orders.status still carried the original CHECK list, which has no
--      'completed'. sp_complete_sales_order (20260305000011, reaffirmed by
--      20260821000002) sets exactly that value, and sp_create_return only
--      accepts an order in it, so completing an order violated the constraint.

CREATE SEQUENCE IF NOT EXISTS sales.order_seq START WITH 1000;
CREATE SEQUENCE IF NOT EXISTS sales.return_seq START WITH 1000;

ALTER TABLE sales.sales_orders
    DROP CONSTRAINT IF EXISTS sales_orders_status_check;
ALTER TABLE sales.sales_orders
    ADD CONSTRAINT sales_orders_status_check
    CHECK (status IN ('pending', 'confirmed', 'shipped', 'delivered', 'completed', 'cancelled'));

DROP FUNCTION IF EXISTS sales.sp_create_sales_order(UUID, UUID, VARCHAR, TEXT, DECIMAL);
CREATE FUNCTION sales.sp_create_sales_order(
    p_tenant_id UUID,
    p_customer_id UUID,
    p_shipping_address VARCHAR,
    p_notes TEXT,
    p_discount_amount DECIMAL DEFAULT 0
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
DECLARE
    v_order_id UUID;
    v_order_number VARCHAR;
    v_sequence TEXT;
BEGIN
    -- Validate customer exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM sales.customers c WHERE c.id = p_customer_id AND c.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Generate order number (YYYY-MM-DD-XXXXX format)
    v_sequence := CAST(NEXTVAL('sales.order_seq') AS TEXT);
    v_order_number := TO_CHAR(CURRENT_DATE, 'YYYY-MM-DD') || '-' ||
                      LPAD(v_sequence, GREATEST(5, LENGTH(v_sequence)), '0');

    -- Insert order with pending status; totals stay at 0 until items are added
    INSERT INTO sales.sales_orders (
        tenant_id, customer_id, order_number, shipping_address, notes, discount_amount
    ) VALUES (
        p_tenant_id, p_customer_id, v_order_number, TRIM(p_shipping_address), p_notes,
        COALESCE(p_discount_amount, 0)
    ) RETURNING sales_orders.id INTO v_order_id;

    -- Return created order
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
    FROM sales.sales_orders so
    WHERE so.id = v_order_id AND so.tenant_id = p_tenant_id;
END;
$$;
COMMENT ON FUNCTION sales.sp_create_sales_order IS 'Creates a pending sales order with a sequence-backed order number scoped to a tenant';

DROP FUNCTION IF EXISTS sales.sp_create_return(UUID, UUID, VARCHAR);
CREATE FUNCTION sales.sp_create_return(
    p_tenant_id UUID,
    p_order_id UUID,
    p_reason VARCHAR
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
DECLARE
    v_return_id UUID;
    v_return_number VARCHAR;
    v_sequence TEXT;
    v_order_record RECORD;
BEGIN
    -- Validate order exists, belongs to tenant, and is completed
    SELECT o.id, o.customer_id, o.total INTO v_order_record
    FROM sales.sales_orders o
    WHERE o.id = p_order_id AND o.tenant_id = p_tenant_id AND o.status = 'completed';

    IF v_order_record IS NULL THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Generate return number (RET-YYYY-MM-DD-XXXXX format)
    v_sequence := CAST(NEXTVAL('sales.return_seq') AS TEXT);
    v_return_number := 'RET-' || TO_CHAR(CURRENT_DATE, 'YYYY-MM-DD') || '-' ||
                       LPAD(v_sequence, GREATEST(5, LENGTH(v_sequence)), '0');

    -- Create return record (returns is a child table, accessed through order)
    INSERT INTO sales.returns (
        order_id, customer_id, return_number, total_amount, reason, status
    ) VALUES (
        p_order_id, v_order_record.customer_id, v_return_number,
        v_order_record.total, p_reason, 'pending'
    ) RETURNING returns.id INTO v_return_id;

    -- Return created return record
    RETURN QUERY
    SELECT
        CAST(r.id AS UUID),
        CAST(r.order_id AS UUID),
        CAST(r.customer_id AS UUID),
        CAST(r.return_number AS VARCHAR),
        CAST(r.total_amount AS DECIMAL),
        CAST(r.reason AS TEXT),
        CAST(r.status AS VARCHAR),
        CAST(r.created_at AS TIMESTAMP),
        CAST(r.updated_at AS TIMESTAMP)
    FROM sales.returns r
    WHERE r.id = v_return_id;
END;
$$;
COMMENT ON FUNCTION sales.sp_create_return IS 'Creates a pending return for a completed order with a sequence-backed return number';
