CREATE OR REPLACE FUNCTION inventory.fn_product_search_text(p_tenant_id uuid, p_product_id uuid, p_name text, p_sku text, p_attributes jsonb)
 RETURNS text
 LANGUAGE sql
 STABLE
AS $function$
    SELECT inventory.fn_fold(concat_ws(' ', p_name, p_sku,
        (SELECT string_agg(s.sku, ' ') FROM inventory.product_skus s WHERE s.product_id = p_product_id AND s.tenant_id = p_tenant_id),
        (SELECT string_agg(sc.code, ' ') FROM inventory.sku_codes sc
         INNER JOIN inventory.product_skus s ON s.id = sc.sku_id
         WHERE s.product_id = p_product_id AND sc.tenant_id = p_tenant_id),
        (SELECT string_agg(v, ' ') FROM jsonb_each(p_attributes) e, jsonb_array_elements_text(e.value) v)
    ));
$function$;

COMMENT ON FUNCTION inventory.fn_product_search_text(uuid,uuid,text,text,jsonb) IS 'What a search reads of a product: name, codes and attribute values, without case or accents (INV-024 D4).';
