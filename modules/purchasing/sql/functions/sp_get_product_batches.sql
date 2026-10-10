CREATE OR REPLACE FUNCTION purchasing.sp_get_product_batches(p_tenant_id uuid, p_product_id uuid)
 RETURNS TABLE(id uuid, product_id uuid, batch_number character varying, quantity integer, unit_cost numeric, receipt_date timestamp without time zone, expiration_date timestamp without time zone, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT pb.id, pb.product_id, pb.batch_number, pb.quantity, pb.unit_cost, pb.receipt_date,
           pb.expiration_date, pb.status, pb.created_at, pb.updated_at
    FROM purchasing.product_batches pb
    WHERE pb.tenant_id = p_tenant_id AND pb.product_id = p_product_id AND pb.status != 'expired'
    ORDER BY pb.receipt_date ASC;
END;
$function$;
