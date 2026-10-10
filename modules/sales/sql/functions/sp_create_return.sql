CREATE OR REPLACE FUNCTION sales.sp_create_return(p_tenant_id uuid, p_order_id uuid, p_reason character varying)
 RETURNS TABLE(id uuid, order_id uuid, customer_id uuid, return_number character varying, total_amount numeric, reason text, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION sales.sp_create_return(uuid,uuid,character varying) IS 'Creates a pending return for a completed order with a sequence-backed return number';
