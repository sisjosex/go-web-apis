CREATE OR REPLACE FUNCTION purchasing.sp_get_purchase_orders_count(p_tenant_id uuid, p_status character varying)
 RETURNS TABLE(total_count integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF p_status IS NOT NULL THEN
        RETURN QUERY
        SELECT CAST(COUNT(*) AS INT)
        FROM purchasing.purchase_orders po
        WHERE po.tenant_id = p_tenant_id AND po.status = p_status;
    ELSE
        RETURN QUERY
        SELECT CAST(COUNT(*) AS INT)
        FROM purchasing.purchase_orders po
        WHERE po.tenant_id = p_tenant_id;
    END IF;
END;
$function$;
