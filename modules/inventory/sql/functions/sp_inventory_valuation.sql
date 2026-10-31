CREATE OR REPLACE FUNCTION inventory.sp_inventory_valuation(p_tenant_id uuid, p_category_id uuid DEFAULT NULL::uuid, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(product_id uuid, sku character varying, name character varying, quantity numeric, unit_cost numeric, value numeric, total_value numeric, uncosted_count bigint, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    WITH skus AS (
        SELECT ps.product_id, ps.sku_id, ps.current_quantity AS quantity,
               inventory.fn_sku_unit_cost(p_tenant_id, ps.sku_id) AS cost
        FROM inventory.product_stock ps
        INNER JOIN inventory.products p ON p.id = ps.product_id AND p.tenant_id = p_tenant_id
        WHERE ps.current_quantity > 0
          AND (p_category_id IS NULL OR EXISTS (
              SELECT 1 FROM inventory.product_category_mapping m
              WHERE m.product_id = p.id AND m.category_id = p_category_id
          ))
    ), products AS (
        -- A combination without a cost does not add to the value; the product is marked only when none has one.
        SELECT s.product_id, SUM(s.quantity) AS quantity,
               SUM(s.quantity * s.cost) AS value,
               SUM(s.quantity * s.cost) / NULLIF(SUM(s.quantity) FILTER (WHERE s.cost IS NOT NULL), 0) AS cost
        FROM skus s
        GROUP BY s.product_id
    ), totals AS (
        SELECT COALESCE(SUM(pr.value), 0) AS total_value, COUNT(*) FILTER (WHERE pr.value IS NULL) AS uncosted, COUNT(*) AS total
        FROM products pr
    ), page AS (
        SELECT pr.* FROM products pr
        ORDER BY pr.value DESC NULLS LAST, pr.product_id
        LIMIT p_page_size OFFSET (p_page - 1) * p_page_size
    )
    -- One row with a NULL id on an empty page, so the totals keep their values.
    SELECT p.id, CAST(p.sku AS VARCHAR), CAST(p.name AS VARCHAR), pg.quantity, ROUND(pg.cost, 2), ROUND(pg.value, 2),
           ROUND(t.total_value, 2), t.uncosted, t.total
    FROM totals t
    LEFT JOIN page pg ON TRUE
    LEFT JOIN inventory.products p ON p.id = pg.product_id
    ORDER BY pg.value DESC NULLS LAST, pg.product_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_inventory_valuation(uuid,uuid,integer,integer) IS 'What a tenant''s stock is worth at cost: a page of products, highest value first, with the total and how many have no cost (INV-022).';
