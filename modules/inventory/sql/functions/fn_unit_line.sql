CREATE OR REPLACE FUNCTION inventory.fn_unit_line(p_tenant_id uuid, p_product_id uuid, p_unit_id uuid, p_qty numeric, p_use character varying DEFAULT 'sale'::character varying, p_sku_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(product_unit_id uuid, factor numeric, base_quantity numeric, price numeric)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_unit inventory.product_units%ROWTYPE;
BEGIN
    SELECT pu.* INTO v_unit
    FROM inventory.product_units pu
    WHERE pu.tenant_id = p_tenant_id AND pu.product_id = p_product_id AND pu.is_active
      AND (
          pu.id = p_unit_id
          OR (p_unit_id IS NULL AND ((p_use = 'sale' AND pu.is_sale_default) OR (p_use = 'purchase' AND pu.is_purchase_default) OR pu.is_base))
      )
    -- With no unit named: the default of its use, else the base.
    ORDER BY (pu.id = p_unit_id) DESC NULLS LAST,
             (CASE WHEN p_use = 'sale' THEN pu.is_sale_default ELSE pu.is_purchase_default END) DESC,
             pu.is_base DESC
    LIMIT 1;

    IF v_unit.id IS NULL THEN
        RAISE EXCEPTION 'inventory.unit.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF NOT v_unit.allow_fraction AND p_qty <> TRUNC(p_qty) THEN
        RAISE EXCEPTION 'inventory.unit.no-fraction' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT v_unit.id, v_unit.factor, p_qty * v_unit.factor,
           CAST(COALESCE(v_unit.price, (p.base_price + COALESCE(s.price_modifier, 0)) * v_unit.factor) AS NUMERIC)
    FROM inventory.products p
    LEFT JOIN inventory.product_skus s ON s.id = p_sku_id
    WHERE p.id = p_product_id;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_unit_line(uuid,uuid,uuid,numeric,character varying,uuid) IS 'A sale or purchase line in a presentation: its factor, the quantity in base units and its price per D3, the presentation''s own or (base + the combination''s modifier) × factor. Raises inventory.unit.not-found and inventory.unit.no-fraction.';
