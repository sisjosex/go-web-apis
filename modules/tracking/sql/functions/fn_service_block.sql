CREATE OR REPLACE FUNCTION tracking.fn_service_block(p_tenant_id uuid, p_vehicle_id uuid, p_driver_id uuid)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_reason VARCHAR;
BEGIN
    v_reason := tracking.fn_service_block_reason(p_tenant_id, 'vehicle', p_vehicle_id);
    IF v_reason IS NOT NULL THEN
        RETURN jsonb_build_object('subject', 'vehicle', 'reason', v_reason);
    END IF;
    v_reason := tracking.fn_service_block_reason(p_tenant_id, 'driver', p_driver_id);
    IF v_reason IS NOT NULL THEN
        RETURN jsonb_build_object('subject', 'driver', 'reason', v_reason);
    END IF;
    RETURN NULL;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_service_block(uuid,uuid,uuid) IS 'A trip''s { subject: vehicle|driver, reason } for the first of its vehicle and driver that is blocked, else NULL — read, never stored, so renewing the document clears it without replanning; a function in the target list so a paged list evaluates it after LIMIT (TRACK-064 D2)';
