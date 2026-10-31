CREATE OR REPLACE FUNCTION inventory.sp_product_facets(p_tenant_id uuid, p_category_id uuid, p_search character varying, p_attrs jsonb)
 RETURNS TABLE(code character varying, name character varying, options jsonb)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_filter JSONB := inventory.fn_attribute_filter(p_tenant_id, p_attrs);
BEGIN
    -- Each attribute counts over the other attributes' filters, so a second brand can still be ticked.
    RETURN QUERY
    WITH hits AS (
        SELECT e.key AS code, v AS value
        FROM inventory.fn_products_in_scope(p_tenant_id, p_category_id, p_search, NULL) s
        INNER JOIN inventory.products p ON p.id = s.id AND p.status = 'active'
        CROSS JOIN LATERAL jsonb_each(p.attributes) e
        CROSS JOIN LATERAL jsonb_array_elements_text(e.value) v
        WHERE NOT EXISTS (
            SELECT 1 FROM jsonb_each(v_filter) f
            WHERE f.key <> e.key
              AND NOT COALESCE(p.attributes -> f.key ?| ARRAY(SELECT jsonb_array_elements_text(f.value)), FALSE)
        )
    ), counted AS (
        SELECT h.code, h.value, COUNT(*) AS n FROM hits h GROUP BY h.code, h.value
    )
    SELECT a.code, a.name, jsonb_agg(jsonb_build_object('value', c.value, 'count', c.n) ORDER BY c.n DESC, c.value)
    FROM counted c
    INNER JOIN inventory.attributes a ON a.tenant_id = p_tenant_id AND a.code = c.code AND a.filterable
    GROUP BY a.code, a.name
    ORDER BY a.name;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_product_facets(uuid,uuid,character varying,jsonb) IS 'What the list''s filters offer: per filterable attribute, its values with how many active products have each (INV-024).';
