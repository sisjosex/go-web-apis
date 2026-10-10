CREATE OR REPLACE FUNCTION purchasing.sp_delete_supplier(p_tenant_id uuid, p_supplier_id uuid)
 RETURNS TABLE(deleted boolean)
 LANGUAGE plpgsql
AS $function$
BEGIN
    UPDATE purchasing.suppliers s
    SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP
    WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id;

    RETURN QUERY SELECT TRUE;
END;
$function$;
