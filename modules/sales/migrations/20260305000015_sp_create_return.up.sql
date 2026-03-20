-- Create a return request from a completed order
CREATE OR REPLACE FUNCTION sales.sp_create_return(
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
