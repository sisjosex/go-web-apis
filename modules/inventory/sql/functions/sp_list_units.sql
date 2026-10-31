CREATE OR REPLACE FUNCTION inventory.sp_list_units(p_tenant_id uuid)
 RETURNS TABLE(id uuid, code character varying, name_es character varying, name_en character varying, symbol character varying, siat_code character varying, allow_fraction boolean)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT u.id, u.code, u.name_es, u.name_en, u.symbol, u.siat_code, u.allow_fraction
    FROM inventory.units u
    WHERE u.tenant_id IS NULL OR u.tenant_id = p_tenant_id
    ORDER BY u.sort_order, u.name_es;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_units(uuid) IS 'The units a business can use: the standard list and its own (INV-023 D2).';
