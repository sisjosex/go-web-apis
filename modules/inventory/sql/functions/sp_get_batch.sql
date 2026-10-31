CREATE OR REPLACE FUNCTION inventory.sp_get_batch(p_tenant_id uuid, p_today date, p_batch_id uuid)
 RETURNS TABLE(id uuid, product_id uuid, lot_number character varying, purchase_date date, expiry_date date, unit_cost numeric, initial_quantity numeric, current_quantity numeric, status character varying, days_to_expiry integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.lot_number,
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        inventory.fn_batch_status(pb.expiry_date, pb.status, p_today),
        pb.expiry_date - p_today
    FROM inventory.product_batches pb
    WHERE pb.tenant_id = p_tenant_id AND pb.id = p_batch_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_batch(uuid,date,uuid) IS 'One lot of the tenant, with its status and days to expiry on p_today, the tenant''s own date (INV-018 D1).';
