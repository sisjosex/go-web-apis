CREATE OR REPLACE FUNCTION inventory.sp_update_batch(p_tenant_id uuid, p_today date, p_batch_id uuid, p_lot_number character varying DEFAULT NULL::character varying, p_expiry_date date DEFAULT NULL::date, p_updated_by uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, product_id uuid, sku_id uuid, sku character varying, lot_number character varying, purchase_date date, expiry_date date, unit_cost numeric, initial_quantity numeric, current_quantity numeric, status character varying, days_to_expiry integer, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_status      VARCHAR;
    v_lot_number  VARCHAR;
    v_expiry_date DATE;
BEGIN
    SELECT pb.status, pb.lot_number, pb.expiry_date
    INTO v_status, v_lot_number, v_expiry_date
    FROM inventory.product_batches pb
    WHERE pb.id = p_batch_id
      AND pb.tenant_id = p_tenant_id
    FOR UPDATE;

    IF v_status IS NULL
    THEN
        RAISE EXCEPTION 'batch.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_status = 'void'
    THEN
        RAISE EXCEPTION 'inventory.batch.voided' USING ERRCODE = 'P0001';
    END IF;

    -- The money guard: consumption has already priced this lot into sales.
    IF EXISTS (
        SELECT 1
        FROM inventory.batch_movements bm
        WHERE bm.batch_id = p_batch_id
    )
    THEN
        RAISE EXCEPTION 'inventory.batch.has-movements' USING ERRCODE = 'P0001';
    END IF;

    -- NULL means "leave it alone", so a caller can correct one field alone.
    IF p_lot_number IS NOT NULL
    THEN
        IF TRIM(p_lot_number) = ''
        THEN
            RAISE EXCEPTION 'batch.lot-number-required' USING ERRCODE = 'P0001';
        END IF;

        v_lot_number := TRIM(p_lot_number);
    END IF;

    IF p_expiry_date IS NOT NULL
    THEN
        IF p_expiry_date <= p_today
        THEN
            RAISE EXCEPTION 'batch.expiry-date-must-be-future'
                USING ERRCODE = 'P0001';
        END IF;

        v_expiry_date := p_expiry_date;
    END IF;

    v_status := inventory.fn_batch_status(v_expiry_date, v_status, p_today);

    UPDATE inventory.product_batches pb
    SET lot_number  = v_lot_number,
        expiry_date = v_expiry_date,
        status      = v_status,
        updated_by  = p_updated_by,
        updated_at  = CURRENT_TIMESTAMP
    WHERE pb.id = p_batch_id;

    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.sku_id,
        CAST(s.sku AS VARCHAR),
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(pb.status AS VARCHAR),
        CAST(pb.expiry_date - p_today AS INT),
        CAST('Batch updated successfully' AS TEXT)
    FROM inventory.product_batches pb
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.id = p_batch_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_update_batch(uuid,date,uuid,character varying,date,uuid) IS 'Corrects a lot''s lot_number and expiry_date, and nothing else (INV-014 D3). A NULL argument leaves that field as it is. Refuses with inventory.batch.has-movements once the lot has been consumed, because batch_movements already carries the COGS booked against it, and with inventory.batch.voided on a lot that was written off.';
