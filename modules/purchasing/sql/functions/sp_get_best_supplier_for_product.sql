CREATE OR REPLACE FUNCTION purchasing.sp_get_best_supplier_for_product(p_product_id uuid, p_tenant_id uuid)
 RETURNS TABLE(id uuid, name character varying, contact_person character varying, email character varying, phone character varying, address text, payment_terms integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        s.name,
        s.contact_person,
        s.email,
        s.phone,
        s.address,
        s.payment_terms,
        s.is_active,
        s.created_at,
        s.updated_at
    FROM purchasing.suppliers s
    JOIN purchasing.supplier_rates sr ON s.id = sr.supplier_id
    WHERE sr.product_id = p_product_id AND s.tenant_id = p_tenant_id
    ORDER BY sr.unit_cost ASC
    LIMIT 1;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_get_best_supplier_for_product(uuid,uuid) IS 'Returns the supplier with the lowest unit cost for a given product, scoped to the tenant.';
