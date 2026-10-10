CREATE OR REPLACE FUNCTION inventory.sp_get_product(p_tenant_id uuid, p_product_id uuid)
 RETURNS TABLE(id uuid, sku character varying, name character varying, description text, base_price numeric, has_variants boolean, status character varying, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        p.id,
        p.sku,
        p.name,
        p.description,
        p.base_price,
        p.has_variants,
        p.status,
        p.created_at
    FROM inventory.products p
    WHERE p.tenant_id = p_tenant_id AND p.id = p_product_id AND p.status = 'active';
END;
$function$;
