CREATE OR REPLACE FUNCTION inventory.sp_set_count_status(p_tenant_id uuid, p_count_id uuid, p_action character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count inventory.stock_counts%ROWTYPE;
    v_next  VARCHAR;
BEGIN
    SELECT sc.* INTO v_count
    FROM inventory.stock_counts sc
    WHERE sc.id = p_count_id AND sc.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_count.id IS NULL THEN
        RAISE EXCEPTION 'inventory.count.not-found' USING ERRCODE = 'P0001';
    END IF;
    v_next := CASE
        WHEN p_action = 'pause' AND v_count.status = 'open' THEN 'paused'
        WHEN p_action = 'resume' AND v_count.status IN ('paused', 'review') THEN 'open'
        WHEN p_action = 'submit' AND v_count.status = 'open' THEN 'review'
        WHEN p_action = 'cancel' AND v_count.status IN ('open', 'paused', 'review') THEN 'cancelled'
    END;
    IF v_next IS NULL THEN
        RAISE EXCEPTION 'inventory.count.invalid-state' USING ERRCODE = 'P0001';
    END IF;
    IF p_action = 'submit' AND EXISTS (
        SELECT 1 FROM inventory.stock_count_lines l WHERE l.count_id = p_count_id AND l.counted IS NULL
    ) THEN
        RAISE EXCEPTION 'inventory.count.incomplete' USING ERRCODE = 'P0001';
    END IF;
    UPDATE inventory.stock_counts sc SET status = v_next WHERE sc.id = p_count_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_set_count_status(uuid,uuid,character varying) IS 'Pauses, resumes, sends to review or cancels a count (INV-026 D4). Raises inventory.count.not-found, inventory.count.invalid-state, inventory.count.incomplete.';
