CREATE OR REPLACE FUNCTION inventory.sp_reorder_categories(p_tenant_id uuid, p_items jsonb)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_ids UUID[];
BEGIN
    SELECT array_agg((e ->> 'id')::UUID) INTO v_ids FROM jsonb_array_elements(p_items) e;

    IF EXISTS (
        SELECT 1 FROM unnest(v_ids) x
        WHERE NOT EXISTS (
            SELECT 1 FROM inventory.product_categories pc
            WHERE pc.id = x AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL
        )
    ) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF EXISTS (
        SELECT 1 FROM jsonb_array_elements(p_items) e
        WHERE e ->> 'parent_id' IS NOT NULL AND NOT EXISTS (
            SELECT 1 FROM inventory.product_categories pc
            WHERE pc.id = (e ->> 'parent_id')::UUID AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL
        )
    ) THEN
        RAISE EXCEPTION 'category.parent-not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE inventory.product_categories pc
    SET parent_id = (e ->> 'parent_id')::UUID, display_order = (e ->> 'display_order')::INT, updated_at = NOW()
    FROM jsonb_array_elements(p_items) e
    WHERE pc.id = (e ->> 'id')::UUID AND pc.tenant_id = p_tenant_id;

    -- After the move: a row that reaches itself going up is a cycle, and the exception undoes the update.
    IF EXISTS (
        WITH RECURSIVE up AS (
            SELECT pc.id AS origin, pc.parent_id AS current, 1 AS depth
            FROM inventory.product_categories pc
            WHERE pc.id = ANY (v_ids) AND pc.parent_id IS NOT NULL
            UNION ALL
            SELECT u.origin, pc.parent_id, u.depth + 1
            FROM up u
            INNER JOIN inventory.product_categories pc ON pc.id = u.current
            WHERE pc.parent_id IS NOT NULL AND u.depth < 64 AND u.current <> u.origin
        )
        SELECT 1 FROM up u WHERE u.current = u.origin
    ) THEN
        RAISE EXCEPTION 'category.circular-hierarchy' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_reorder_categories(uuid,jsonb) IS 'Moves several categories in one statement, parent and place, refusing a cycle. Raises category.not-found, category.parent-not-found, category.circular-hierarchy.';
