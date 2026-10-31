CREATE OR REPLACE FUNCTION inventory.sp_get_category_attributes(p_tenant_id uuid, p_category_id uuid)
 RETURNS TABLE(id uuid, code character varying, name character varying, type character varying, unit character varying, axis boolean, options jsonb, inherited boolean)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM inventory.product_categories pc
        WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH RECURSIVE line AS (
        SELECT pc.id, pc.parent_id, 0 AS depth
        FROM inventory.product_categories pc
        WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id
        UNION ALL
        SELECT pc.id, pc.parent_id, l.depth + 1
        FROM inventory.product_categories pc
        INNER JOIN line l ON pc.id = l.parent_id
        WHERE l.depth < 6 AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL
    ), nearest AS (
        SELECT ca.attribute_id, MIN(l.depth) AS depth
        FROM line l
        INNER JOIN inventory.category_attributes ca ON ca.category_id = l.id
        GROUP BY ca.attribute_id
    )
    SELECT
        a.id, a.code, a.name, a.type, a.unit, a.axis,
        (
            SELECT COALESCE(jsonb_agg(jsonb_build_object('id', o.id, 'value', o.value, 'swatch', o.swatch, 'sort', o.sort_order) ORDER BY o.sort_order, o.value), '[]'::jsonb)
            FROM inventory.attribute_options o
            WHERE o.attribute_id = a.id
        ),
        n.depth > 0
    FROM nearest n
    INNER JOIN inventory.attributes a ON a.id = n.attribute_id
    ORDER BY n.depth DESC, a.name;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_category_attributes(uuid,uuid) IS 'The attributes of a category: its own and those it inherits, up to 6 levels up (INV-024 D3).';
