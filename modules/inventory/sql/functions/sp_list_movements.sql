CREATE OR REPLACE FUNCTION inventory.sp_list_movements(p_tenant_id uuid, p_product_id uuid DEFAULT NULL::uuid, p_movement_type character varying DEFAULT NULL::character varying, p_date_from date DEFAULT NULL::date, p_date_to date DEFAULT NULL::date, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_sku_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, product_id uuid, movement_type character varying, quantity numeric, reference_type character varying, reference_id uuid, unit_cost numeric, notes text, created_by uuid, created_at timestamp without time zone, sku_id uuid, sku character varying, total_count bigint)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_offset      INT;
    v_total_count BIGINT;
BEGIN
    v_offset := (p_page - 1) * p_page_size;

    SELECT COUNT(*)
    INTO v_total_count
    FROM inventory.inventory_movements m
    WHERE m.tenant_id = p_tenant_id
      AND (p_product_id    IS NULL OR m.product_id    = p_product_id)
      AND (p_movement_type IS NULL OR m.movement_type = p_movement_type)
      AND (p_sku_id        IS NULL OR m.sku_id        = p_sku_id)
      AND (p_date_from     IS NULL OR m.created_at   >= CAST(p_date_from AS TIMESTAMP))
      -- p_date_to covers its whole day, so from = to on the same date still matches.
      AND (p_date_to       IS NULL OR m.created_at    < CAST(p_date_to + 1 AS TIMESTAMP));

    RETURN QUERY
    SELECT
        m.id,
        m.product_id,
        m.movement_type,
        m.quantity,
        m.reference_type,
        m.reference_id,
        m.unit_cost,
        m.notes,
        m.created_by,
        m.created_at,
        m.sku_id,
        -- LEFT JOIN: sku_id is nullable on rows written before INV-007 and the
        -- trail must keep showing them.
        CAST(s.sku AS VARCHAR),
        CAST(v_total_count AS BIGINT)
    FROM inventory.inventory_movements m
    LEFT JOIN inventory.product_skus s ON s.id = m.sku_id
    WHERE m.tenant_id = p_tenant_id
      AND (p_product_id    IS NULL OR m.product_id    = p_product_id)
      AND (p_movement_type IS NULL OR m.movement_type = p_movement_type)
      AND (p_sku_id        IS NULL OR m.sku_id        = p_sku_id)
      AND (p_date_from     IS NULL OR m.created_at   >= CAST(p_date_from AS TIMESTAMP))
      AND (p_date_to       IS NULL OR m.created_at    < CAST(p_date_to + 1 AS TIMESTAMP))
    -- m.id breaks ties: two movements written inside one transaction share created_at.
    ORDER BY m.created_at DESC, m.id DESC
    LIMIT  p_page_size
    OFFSET v_offset;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_movements(uuid,uuid,character varying,date,date,integer,integer,uuid) IS 'Lists a tenant''s inventory movements newest first, optionally filtered by product, combination, type and date range, with the combination''s SKU code and total_count on every row';
