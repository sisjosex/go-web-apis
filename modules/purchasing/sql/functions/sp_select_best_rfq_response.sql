CREATE OR REPLACE FUNCTION purchasing.sp_select_best_rfq_response(p_tenant_id uuid, p_rfq_id uuid, p_response_id uuid)
 RETURNS TABLE(status_updated boolean)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM purchasing.rfq_responses rr WHERE rr.id = p_response_id AND rr.rfq_id = p_rfq_id) THEN
        RAISE EXCEPTION 'rfq.response.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE purchasing.rfq_responses rr
    SET selected = (rr.id = p_response_id),
        updated_at = CURRENT_TIMESTAMP
    WHERE rr.rfq_id = p_rfq_id;

    UPDATE purchasing.request_for_quotes r
    SET status = 'closed',
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id;

    RETURN QUERY SELECT TRUE;
END;
$function$;
