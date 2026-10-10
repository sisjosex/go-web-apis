CREATE OR REPLACE FUNCTION purchasing.sp_get_price_comparison(p_product_id uuid, p_tenant_id uuid)
 RETURNS TABLE(product_id uuid, supplier_id uuid, supplier_name character varying, unit_cost numeric, payment_terms integer, last_price_date timestamp without time zone, average_cost numeric)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        sr.product_id,
        sr.supplier_id,
        s.name,
        sr.unit_cost,
        s.payment_terms,
        sr.last_price_date,
        AVG(sr.unit_cost) OVER (PARTITION BY sr.product_id)
    FROM purchasing.supplier_rates sr
    JOIN purchasing.suppliers s ON s.id = sr.supplier_id
    WHERE sr.product_id = p_product_id AND s.tenant_id = p_tenant_id
    ORDER BY sr.unit_cost ASC;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_get_price_comparison(uuid,uuid) IS 'Returns all supplier prices for a product with computed average cost, ordered by unit cost ascending.';
