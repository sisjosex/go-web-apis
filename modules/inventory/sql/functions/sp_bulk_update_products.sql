CREATE OR REPLACE FUNCTION inventory.sp_bulk_update_products(p_tenant_id uuid, p_ids uuid[], p_status character varying DEFAULT NULL::character varying, p_base_price numeric DEFAULT NULL::numeric, p_price_change_pct numeric DEFAULT NULL::numeric, p_category_id uuid DEFAULT NULL::uuid, p_pos_visible boolean DEFAULT NULL::boolean, p_updated_by uuid DEFAULT NULL::uuid)
 RETURNS TABLE(updated integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_ids UUID[];
BEGIN
    IF p_category_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM inventory.product_categories pc
        WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    WITH changed AS (
        UPDATE inventory.products p
        SET status      = COALESCE(p_status, p.status),
            base_price  = COALESCE(p_base_price, ROUND(p.base_price * (1 + p_price_change_pct / 100), 2), p.base_price),
            pos_visible = COALESCE(p_pos_visible, p.pos_visible),
            updated_by  = p_updated_by,
            updated_at  = CURRENT_TIMESTAMP
        WHERE p.tenant_id = p_tenant_id
          AND p.id = ANY (p_ids)
        RETURNING p.id
    )
    SELECT COALESCE(array_agg(c.id), ARRAY[]::UUID[]) INTO v_ids FROM changed c;

    IF p_category_id IS NOT NULL THEN
        DELETE FROM inventory.product_category_mapping pcm WHERE pcm.product_id = ANY (v_ids);
        INSERT INTO inventory.product_category_mapping (product_id, category_id)
        SELECT product_id, p_category_id FROM unnest(v_ids) AS product_id;
    END IF;

    RETURN QUERY SELECT CAST(COALESCE(array_length(v_ids, 1), 0) AS INT);
END;
$function$;

COMMENT ON FUNCTION inventory.sp_bulk_update_products(uuid,uuid[],character varying,numeric,numeric,uuid,boolean,uuid) IS 'One change to several products of the tenant in one statement: status, a price or a percent change, the one category replacing theirs, or the POS flag (INV-019). Ids of another tenant are ignored; answers how many changed. Raises category.not-found.';
