CREATE OR REPLACE FUNCTION billing.sp_reject_payment(p_payment_id uuid, p_reason text, p_rejected_by uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, user_id uuid, plan character varying, billing_cycle character varying, amount numeric, currency character varying, status character varying, reference character varying, paid_at timestamp with time zone, period_start timestamp with time zone, period_end timestamp with time zone, created_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_pay billing.payments%ROWTYPE;
BEGIN
    SELECT * INTO v_pay FROM billing.payments p WHERE p.id = p_payment_id FOR UPDATE;
    IF v_pay.id IS NULL OR v_pay.tenant_id IS NULL THEN
        RAISE EXCEPTION 'billing.payment.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_pay.status <> 'notified' THEN
        RAISE EXCEPTION 'billing.payment.not-pending' USING ERRCODE = 'P0001';
    END IF;

    UPDATE billing.payments p SET status = 'rejected', reject_reason = NULLIF(TRIM(p_reason), ''),
        rejected_by = p_rejected_by
    WHERE p.id = v_pay.id;

    RETURN QUERY SELECT * FROM billing.fn_payment_row(v_pay.id);
END;
$function$;

COMMENT ON FUNCTION billing.sp_reject_payment(uuid,text,uuid) IS 'Rejects a notified payment with a reason; the plan does not change (BILLING-002 D3); raises billing.payment.not-found, billing.payment.not-pending';
