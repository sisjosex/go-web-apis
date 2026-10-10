CREATE OR REPLACE FUNCTION purchasing.sp_get_suppliers_count(p_tenant_id uuid)
 RETURNS TABLE(total_count integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT CAST(COUNT(*) AS INT) FROM purchasing.suppliers s
    WHERE s.tenant_id = p_tenant_id AND s.is_active = TRUE;
END;
$function$;
