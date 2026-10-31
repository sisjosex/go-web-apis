CREATE OR REPLACE FUNCTION inventory.fn_product_units(p_tenant_id uuid, p_product_id uuid)
 RETURNS jsonb
 LANGUAGE sql
 STABLE
AS $function$
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'id', pu.id, 'unit_id', pu.unit_id, 'code', u.code, 'name_es', u.name_es, 'name_en', u.name_en, 'symbol', u.symbol,
        'factor', pu.factor, 'is_base', pu.is_base, 'allow_fraction', pu.allow_fraction, 'sellable', pu.sellable,
        'pos_visible', pu.pos_visible, 'is_sale_default', pu.is_sale_default, 'is_purchase_default', pu.is_purchase_default,
        'price', pu.price, 'codes', (
            SELECT COALESCE(jsonb_agg(jsonb_build_object('sku_id', sc.sku_id, 'code', sc.code) ORDER BY sc.code), '[]'::jsonb)
            FROM inventory.sku_codes sc
            WHERE sc.product_unit_id = pu.id
        )
    ) ORDER BY pu.is_base DESC, pu.factor), '[]'::jsonb)
    FROM inventory.product_units pu
    INNER JOIN inventory.units u ON u.id = pu.unit_id
    WHERE pu.tenant_id = p_tenant_id AND pu.product_id = p_product_id AND pu.is_active;
$function$;

COMMENT ON FUNCTION inventory.fn_product_units(uuid,uuid) IS 'A product''s active presentations with their codes, base first, as the product reads answer them (INV-023).';
