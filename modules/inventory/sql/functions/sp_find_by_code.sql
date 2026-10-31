CREATE OR REPLACE FUNCTION inventory.sp_find_by_code(p_tenant_id uuid, p_code character varying)
 RETURNS TABLE(product_id uuid, product_name character varying, sku_id uuid, sku character varying, product_unit_id uuid, unit_name_es character varying, unit_name_en character varying, factor numeric, price numeric)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        p.id,
        CAST(p.name AS VARCHAR),
        s.id,
        CAST(s.sku AS VARCHAR),
        pu.id,
        u.name_es,
        u.name_en,
        pu.factor,
        CAST(COALESCE(pu.price, (p.base_price + s.price_modifier) * pu.factor) AS NUMERIC)
    FROM inventory.sku_codes sc
    INNER JOIN inventory.product_skus s ON s.id = sc.sku_id
    INNER JOIN inventory.product_units pu ON pu.id = sc.product_unit_id AND pu.is_active
    INNER JOIN inventory.units u ON u.id = pu.unit_id
    INNER JOIN inventory.products p ON p.id = s.product_id
    WHERE sc.tenant_id = p_tenant_id AND sc.code = TRIM(p_code);

    IF NOT FOUND THEN
        RAISE EXCEPTION 'inventory.code.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_find_by_code(uuid,character varying) IS 'What a scanned barcode names: product, combination, presentation, factor and price per D3, by the tenant and code index (INV-023 D1). Raises inventory.code.not-found.';
