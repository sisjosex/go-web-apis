-- Approve a return and restore inventory
CREATE OR REPLACE FUNCTION sales.sp_approve_return(
    p_return_id UUID
)
RETURNS TABLE(
    return_id UUID,
    return_number VARCHAR,
    status VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_return_record RECORD;
    v_order_id UUID;
    v_batch_record RECORD;
BEGIN
    -- Get return details
    SELECT id, return_number, order_id, status
    INTO v_return_record
    FROM sales.returns
    WHERE id = p_return_id;

    IF v_return_record IS NULL THEN
        RAISE EXCEPTION 'return.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_return_record.status != 'pending' THEN
        RAISE EXCEPTION 'return.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    v_order_id := v_return_record.order_id;

    -- Restore batch quantities from the order assignments
    FOR v_batch_record IN
        SELECT oba.product_batch_id, oba.quantity_assigned
        FROM sales.order_batch_assignments oba
        WHERE oba.order_id = v_order_id
    LOOP
        UPDATE inventory.product_batches
        SET current_quantity = current_quantity + v_batch_record.quantity_assigned
        WHERE id = v_batch_record.product_batch_id;
    END LOOP;

    -- Update return status to approved
    UPDATE sales.returns
    SET status = 'approved'
    WHERE id = p_return_id;

    RETURN QUERY
    SELECT
        p_return_id,
        v_return_record.return_number,
        'approved'::VARCHAR,
        'Return approved and inventory restored'::TEXT;
END;
$$;
