CREATE OR REPLACE FUNCTION inventory.sp_list_movements(p_tenant_id uuid, p_product_id uuid DEFAULT NULL::uuid, p_movement_type character varying DEFAULT NULL::character varying, p_date_from date DEFAULT NULL::date, p_date_to date DEFAULT NULL::date, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_sku_id uuid DEFAULT NULL::uuid, p_search character varying DEFAULT NULL::character varying, p_reference_type character varying DEFAULT NULL::character varying, p_group character varying DEFAULT 'none'::character varying)
 RETURNS TABLE(id uuid, product_id uuid, movement_type character varying, quantity numeric, reference_type character varying, reference_id uuid, unit_cost numeric, notes text, created_by uuid, created_at timestamp without time zone, sku_id uuid, sku character varying, total_count bigint, product_name character varying, sku_label character varying, created_by_name character varying, reference_label character varying, more_count integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    WITH matched AS (
        SELECT
            m.*,
            -- One document, one row (INV-021 D2): its lines share a key; a movement with no document, or the
            -- kardex of one product, keeps its own.
            CASE
                WHEN p_group = 'document' AND m.reference_id IS NOT NULL
                THEN m.reference_type || ':' || m.reference_id::TEXT || ':' || m.movement_type
                ELSE m.id::TEXT
            END AS doc_key
        FROM inventory.inventory_movements m
        INNER JOIN inventory.products p ON p.id = m.product_id AND p.tenant_id = p_tenant_id
        LEFT JOIN auth.users su ON su.id = m.created_by
        WHERE m.tenant_id = p_tenant_id
          AND (p_product_id     IS NULL OR m.product_id = p_product_id)
          AND (p_movement_type  IS NULL OR p_movement_type = '' OR m.movement_type = ANY (string_to_array(p_movement_type, ',')))
          AND (p_reference_type IS NULL OR p_reference_type = '' OR m.reference_type = p_reference_type)
          AND (p_sku_id         IS NULL OR m.sku_id     = p_sku_id)
          AND (p_date_from      IS NULL OR m.created_at >= CAST(p_date_from AS TIMESTAMP))
          -- p_date_to covers its whole day, so from = to on the same date still matches.
          AND (p_date_to        IS NULL OR m.created_at <  CAST(p_date_to + 1 AS TIMESTAMP))
          AND (
              p_search IS NULL
              OR p_search = ''
              OR p.name ILIKE '%' || p_search || '%'
              OR p.sku  ILIKE '%' || p_search || '%'
              OR m.reference_label ILIKE '%' || p_search || '%'
              OR CONCAT_WS(' ', su.first_name, su.last_name) ILIKE '%' || p_search || '%'
          )
    ),
    grouped AS (
        SELECT
            g.doc_key,
            (array_agg(g.id ORDER BY g.created_at DESC, g.id DESC))[1] AS first_id,
            MAX(g.created_at) AS last_at,
            COUNT(*) AS lines,
            -- A transfer's lines cancel out: what it moved is what came in.
            CASE
                WHEN MIN(g.movement_type) = 'TRANSFER' AND COUNT(*) > 1
                THEN SUM(g.quantity) FILTER (WHERE g.quantity > 0)
                ELSE SUM(g.quantity)
            END AS total_quantity
        FROM matched g
        GROUP BY g.doc_key
    ),
    page AS (
        SELECT gr.*, COUNT(*) OVER () AS total_count
        FROM grouped gr
        -- first_id breaks ties: movements written inside one transaction share created_at.
        ORDER BY gr.last_at DESC, gr.first_id DESC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        m.id,
        m.product_id,
        m.movement_type,
        CAST(pg.total_quantity AS NUMERIC),
        m.reference_type,
        m.reference_id,
        m.unit_cost,
        m.notes,
        m.created_by,
        pg.last_at,
        m.sku_id,
        -- LEFT JOIN: sku_id is nullable on rows written before INV-007 and the
        -- trail must keep showing them.
        CAST(s.sku AS VARCHAR),
        CAST(pg.total_count AS BIGINT),
        CAST(p.name AS VARCHAR),
        CAST((
            SELECT string_agg(vo.option_name, ' · ' ORDER BY vg.sort_order, vg.group_type)
            FROM inventory.product_sku_options so
            INNER JOIN inventory.product_variant_options vo ON vo.id = so.option_id
            INNER JOIN inventory.product_variant_groups vg ON vg.id = so.variant_group_id
            WHERE so.sku_id = m.sku_id
        ) AS VARCHAR),
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR),
        CAST(m.reference_label AS VARCHAR),
        CAST(pg.lines - 1 AS INT)
    FROM page pg
    INNER JOIN inventory.inventory_movements m ON m.id = pg.first_id
    INNER JOIN inventory.products p ON p.id = m.product_id
    LEFT JOIN inventory.product_skus s ON s.id = m.sku_id
    LEFT JOIN auth.users u ON u.id = m.created_by
    ORDER BY pg.last_at DESC, pg.first_id DESC;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_movements(uuid,uuid,character varying,date,date,integer,integer,uuid,character varying,character varying,character varying) IS 'A tenant''s movements newest first, filtered by product, combination, types (comma list), document type, date range and a search over product, document and person. p_group = document folds a document''s lines into one row with more_count and the signed sum (INV-021 D2); none is one row per movement, the kardex. Every row carries product, combination and user names and total_count.';
