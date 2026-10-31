CREATE OR REPLACE FUNCTION purchasing.sp_add_purchase_order_item(p_tenant_id uuid, p_po_id uuid, p_product_id uuid, p_quantity numeric, p_unit_cost numeric, p_product_unit_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(item_id uuid, po_id uuid, product_id uuid, quantity numeric, unit_cost numeric, line_total numeric, message text)
 LANGUAGE plpgsql
AS $function$
#variable_conflict use_column
DECLARE
    v_item_id UUID := gen_random_uuid();
    v_supplier_id UUID;
    v_unit_id UUID;
    v_factor DECIMAL;
BEGIN
    SELECT po.supplier_id INTO v_supplier_id
    FROM purchasing.purchase_orders po
    WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status IN ('draft', 'approved');

    IF v_supplier_id IS NULL THEN
        RAISE EXCEPTION 'po.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'po.invalid-quantity' USING ERRCODE = 'P0001';
    END IF;

    IF p_unit_cost <= 0 THEN
        RAISE EXCEPTION 'po.invalid-cost' USING ERRCODE = 'P0001';
    END IF;

    -- The line keeps what was bought; stock reads its base quantity (INV-023).
    SELECT l.product_unit_id, l.factor INTO v_unit_id, v_factor
    FROM inventory.fn_unit_line(p_tenant_id, p_product_id, p_product_unit_id, p_quantity, 'purchase') l;

    INSERT INTO purchasing.purchase_order_items(id, purchase_order_id, product_id, quantity, unit_cost, product_unit_id, unit_factor)
    VALUES (v_item_id, p_po_id, p_product_id, p_quantity, p_unit_cost, v_unit_id, v_factor);

    UPDATE purchasing.purchase_orders
    SET total_amount = (
            SELECT COALESCE(SUM(poi.line_total), 0)
            FROM purchasing.purchase_order_items poi
            WHERE poi.purchase_order_id = p_po_id
        ),
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    -- The supplier's latest cost for the product: what price comparison ranks.
    INSERT INTO purchasing.supplier_rates (supplier_id, product_id, unit_cost, payment_terms, last_price_date)
    SELECT v_supplier_id, p_product_id, p_unit_cost, s.payment_terms, CURRENT_TIMESTAMP
    FROM purchasing.suppliers s
    WHERE s.id = v_supplier_id
    ON CONFLICT (supplier_id, product_id) DO UPDATE
    SET unit_cost = EXCLUDED.unit_cost,
        last_price_date = EXCLUDED.last_price_date,
        updated_at = CURRENT_TIMESTAMP;

    RETURN QUERY
    SELECT
        v_item_id,
        p_po_id,
        p_product_id,
        p_quantity,
        p_unit_cost,
        (p_quantity * p_unit_cost)::DECIMAL,
        'Item added to purchase order'::TEXT;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_add_purchase_order_item(uuid,uuid,uuid,numeric,numeric,uuid) IS 'Adds a line to a draft order in a presentation, the purchase default when none is named; stock later reads its base quantity (INV-023).';
