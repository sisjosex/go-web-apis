CREATE OR REPLACE FUNCTION sales.sp_create_payment(p_tenant_id uuid, p_order_id uuid, p_amount numeric, p_payment_method character varying, p_reference_number character varying DEFAULT NULL::character varying, p_notes text DEFAULT NULL::text)
 RETURNS TABLE(payment_id uuid, order_id uuid, amount numeric, payment_method character varying, status character varying, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_payment_id UUID := gen_random_uuid();
    v_order_record RECORD;
BEGIN
    -- Validate order exists and belongs to tenant
    SELECT o.id, o.customer_id, o.total INTO v_order_record
    FROM sales.sales_orders o
    WHERE o.id = p_order_id AND o.tenant_id = p_tenant_id;

    IF v_order_record IS NULL THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate amount
    IF p_amount <= 0 THEN
        RAISE EXCEPTION 'payment.invalid-amount' USING ERRCODE = 'P0001';
    END IF;

    -- Validate payment method
    IF p_payment_method NOT IN ('cash', 'card', 'bank_transfer', 'check', 'other') THEN
        RAISE EXCEPTION 'payment.invalid-method' USING ERRCODE = 'P0001';
    END IF;

    -- Create payment record
    INSERT INTO sales.payments(
        id, order_id, customer_id, amount, payment_method,
        reference_number, notes, status
    )
    VALUES(
        v_payment_id, p_order_id, v_order_record.customer_id, p_amount,
        p_payment_method, p_reference_number, p_notes, 'completed'
    );

    RETURN QUERY
    SELECT
        v_payment_id,
        p_order_id,
        p_amount,
        p_payment_method,
        'completed'::VARCHAR,
        'Payment recorded successfully'::TEXT;
END;
$function$;
