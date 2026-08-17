-- sp_list_movements: the trail says which combination each movement belongs to,
-- and can be narrowed to one (INV-011 D2).
--
-- inventory_movements.sku_id has existed since INV-007 and every redistribution
-- has been writing it; this SP was written before it and returned ten columns,
-- none of them the SKU. With INV-011 every movement names a combination, so a
-- trail that cannot show which one is unreadable on a product with axes.
--
-- sku_id and sku are appended after created_at and before total_count, and
-- p_sku_id is appended last so every positional call keeps working. Return type
-- and signature both change, so DROP + CREATE (api/AGENTS.md).

DROP FUNCTION IF EXISTS inventory.sp_list_movements(UUID, UUID, VARCHAR, DATE, DATE, INT, INT);

CREATE FUNCTION inventory.sp_list_movements(
    p_tenant_id     UUID,
    p_product_id    UUID    DEFAULT NULL,
    p_movement_type VARCHAR DEFAULT NULL,
    p_date_from     DATE    DEFAULT NULL,
    p_date_to       DATE    DEFAULT NULL,
    p_page          INT     DEFAULT 1,
    p_page_size     INT     DEFAULT 20,
    p_sku_id        UUID    DEFAULT NULL
)
RETURNS TABLE(
    id             UUID,
    product_id     UUID,
    movement_type  VARCHAR,
    quantity       DECIMAL,
    reference_type VARCHAR,
    reference_id   UUID,
    unit_cost      DECIMAL,
    notes          TEXT,
    created_by     UUID,
    created_at     TIMESTAMP,
    sku_id         UUID,
    sku            VARCHAR,
    total_count    BIGINT
) LANGUAGE plpgsql AS $$
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
$$;

COMMENT ON FUNCTION inventory.sp_list_movements(UUID, UUID, VARCHAR, DATE, DATE, INT, INT, UUID) IS
'Lists a tenant''s inventory movements newest first, optionally filtered by product, combination, type and date range, with the combination''s SKU code and total_count on every row';
