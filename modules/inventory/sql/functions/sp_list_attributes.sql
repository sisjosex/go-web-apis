CREATE OR REPLACE FUNCTION inventory.sp_list_attributes(p_tenant_id uuid, p_attribute_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, code character varying, name character varying, type character varying, unit character varying, filterable boolean, axis boolean, options jsonb, product_count bigint, category_names jsonb)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    WITH used AS (
        SELECT k AS code, COUNT(*) AS n
        FROM inventory.products p, jsonb_object_keys(p.attributes) k
        WHERE p.tenant_id = p_tenant_id
        GROUP BY k
    )
    SELECT
        a.id, a.code, a.name, a.type, a.unit, a.filterable, a.axis,
        (
            SELECT COALESCE(jsonb_agg(jsonb_build_object('id', o.id, 'value', o.value, 'swatch', o.swatch, 'sort', o.sort_order) ORDER BY o.sort_order, o.value), '[]'::jsonb)
            FROM inventory.attribute_options o
            WHERE o.attribute_id = a.id
        ),
        COALESCE(u.n, 0),
        (
            SELECT COALESCE(jsonb_agg(pc.name ORDER BY pc.name), '[]'::jsonb)
            FROM inventory.category_attributes ca
            INNER JOIN inventory.product_categories pc ON pc.id = ca.category_id AND pc.deleted_at IS NULL
            WHERE ca.attribute_id = a.id
        )
    FROM inventory.attributes a
    LEFT JOIN used u ON u.code = a.code
    WHERE a.tenant_id = p_tenant_id AND (p_attribute_id IS NULL OR a.id = p_attribute_id)
    ORDER BY a.name;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_attributes(uuid,uuid) IS 'A tenant''s attributes with their options, how many products use each and which categories bring it; one pass over the products (INV-024).';
