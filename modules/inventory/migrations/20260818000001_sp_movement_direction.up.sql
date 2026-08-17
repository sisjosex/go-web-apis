-- INV-013 D1/D2/D3 — the movement type owns the sign, and the cost is only asked
-- where something was bought.
--
-- Until now sp_record_movement did v_new_stock := v_current_stock + p_quantity
-- and the modal blocked anything <= 0, so a SALE of 5 *raised* stock by 5 and
-- inventory.insufficient-stock could never fire from that path. Nothing else
-- writes movements — sales and purchasing never call this SP — so the convention
-- is still free to fix rather than to migrate around.
--
-- D1: the caller sends a positive quantity plus, for the two types where both
-- readings are legitimate, an explicit direction. Five types decide it
-- themselves and refuse a contradicting one, so the operator cannot type a sale
-- as an inbound movement by mistake and cannot be asked a question that has only
-- one answer.
--
-- D3: unit_cost is a cost. It is required and > 0 on PURCHASE and PRODUCTION,
-- and forced to NULL on every other type — AC-5 is a claim about what is stored,
-- so it is enforced here and not only by the field the modal decides to render.
--
-- p_direction is appended with a DEFAULT and a Postgres function cannot gain a
-- parameter by replacement, so the ten-argument signature is dropped first, the
-- way 20260812000004 already did for the nine-argument one.

-- ===========================================================================
-- fn_movement_direction — one table for the sign, shared by SP and app
-- ===========================================================================
-- Returns +1 or -1, never 0: a movement that moves nothing is not a direction
-- this can express, and the quantity binding refuses it before we get here.
--
-- IMMUTABLE and reading no table: the type/direction table is a rule, not tenant
-- data, so it stays cheap enough to call inline and identical for every caller.
CREATE OR REPLACE FUNCTION inventory.fn_movement_direction(
    p_movement_type VARCHAR,
    p_direction     VARCHAR DEFAULT NULL
) RETURNS INTEGER
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    v_decided VARCHAR;
BEGIN
    -- The five types whose direction is not a question.
    v_decided := CASE p_movement_type
        WHEN 'PURCHASE'   THEN 'IN'
        WHEN 'RETURN'     THEN 'IN'
        WHEN 'PRODUCTION' THEN 'IN'
        WHEN 'SALE'       THEN 'OUT'
        WHEN 'WASTE'      THEN 'OUT'
        ELSE NULL
    END;

    IF v_decided IS NOT NULL THEN
        -- A direction that agrees is accepted rather than refused: the modal
        -- shows the decided one as read-only text and may well send it back.
        IF p_direction IS NOT NULL AND p_direction <> v_decided THEN
            RAISE EXCEPTION 'inventory.movement.direction-conflict'
                USING ERRCODE = 'P0001';
        END IF;
        RETURN CASE WHEN v_decided = 'IN' THEN 1 ELSE -1 END;
    END IF;

    -- ADJUSTMENT and TRANSFER: both readings are legitimate, so the caller says
    -- which. Defaulting either one is how a stock count silently goes the wrong
    -- way, so there is no default.
    IF p_direction IS NULL THEN
        RAISE EXCEPTION 'inventory.movement.direction-required'
            USING ERRCODE = 'P0001';
    END IF;

    IF p_direction NOT IN ('IN', 'OUT') THEN
        RAISE EXCEPTION 'inventory.movement.direction-conflict'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN CASE WHEN p_direction = 'IN' THEN 1 ELSE -1 END;
END;
$$;
COMMENT ON FUNCTION inventory.fn_movement_direction IS
    'The sign a movement applies to stock: +1 inbound, -1 outbound (INV-013 D1). '
    'PURCHASE, RETURN and PRODUCTION are always IN; SALE and WASTE always OUT, '
    'and a p_direction contradicting one of those raises '
    'inventory.movement.direction-conflict. ADJUSTMENT and TRANSFER carry no '
    'default and raise inventory.movement.direction-required when p_direction is '
    'NULL. An unknown movement type reaches here as an undirected one — callers '
    'validate the type first.';

-- ===========================================================================
-- sp_record_movement — positive quantity in, signed quantity stored
-- ===========================================================================
DROP FUNCTION IF EXISTS inventory.sp_record_movement(
    UUID, UUID, VARCHAR, DECIMAL, VARCHAR, UUID, DECIMAL, TEXT, UUID, UUID
);
CREATE FUNCTION inventory.sp_record_movement(
    p_tenant_id      UUID,
    p_product_id     UUID,
    p_movement_type  VARCHAR,
    p_quantity       DECIMAL,
    p_reference_type VARCHAR DEFAULT NULL,
    p_reference_id   UUID DEFAULT NULL,
    p_unit_cost      DECIMAL DEFAULT NULL,
    p_notes          TEXT DEFAULT NULL,
    p_created_by     UUID DEFAULT NULL,
    p_sku_id         UUID DEFAULT NULL,
    p_direction      VARCHAR DEFAULT NULL
)
RETURNS TABLE(
    movement_id        UUID,
    new_stock_quantity DECIMAL,
    message            TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_sku_id        UUID;
    v_movement_id   UUID;
    v_new_stock     DECIMAL;
    v_current_stock DECIMAL;
    v_reorder_level DECIMAL;
    v_status        VARCHAR;
    v_sign          INTEGER;
    v_signed_qty    DECIMAL;
    v_unit_cost     DECIMAL;
BEGIN
    -- INV-011 D1 — the combination is required, never defaulted.
    IF p_sku_id IS NULL AND inventory.fn_requires_sku(p_tenant_id, p_product_id) THEN
        RAISE EXCEPTION 'inventory.movement.sku-required' USING ERRCODE = 'P0001';
    END IF;

    -- Validates the product against the tenant and resolves NULL to the default
    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);

    -- Validate movement type before asking for its direction: an unknown type
    -- has no entry in the table and would come back as direction-required,
    -- which names the wrong problem.
    IF p_movement_type NOT IN ('PURCHASE', 'SALE', 'ADJUSTMENT', 'TRANSFER', 'RETURN', 'WASTE', 'PRODUCTION') THEN
        RAISE EXCEPTION 'inventory.invalid-movement-type' USING ERRCODE = 'P0001';
    END IF;

    -- INV-013 D1 — the type decides the sign, the caller only supplies the
    -- magnitude. ABS() rather than a rejection of a negative: a client that
    -- still sends -5 for a SALE means the same movement it always meant.
    v_sign       := inventory.fn_movement_direction(p_movement_type, p_direction);
    v_signed_qty := v_sign * ABS(p_quantity);

    -- INV-013 D2/D3 — unit_cost is what was paid, so it is asked exactly where
    -- something was bought and stored nowhere else.
    IF p_movement_type IN ('PURCHASE', 'PRODUCTION') THEN
        IF p_unit_cost IS NULL OR p_unit_cost <= 0 THEN
            RAISE EXCEPTION 'inventory.movement.unit-cost-required'
                USING ERRCODE = 'P0001';
        END IF;
        v_unit_cost := p_unit_cost;
    ELSE
        v_unit_cost := NULL;
    END IF;

    -- Lock the row before reading it: without this two concurrent SALEs both
    -- pass the insufficient-stock check below.
    SELECT ps.current_quantity, ps.reorder_level
    INTO v_current_stock, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.sku_id = v_sku_id
    FOR UPDATE;

    v_new_stock := v_current_stock + v_signed_qty;

    -- Every outbound type, not only SALE: WASTE and an ADJUSTMENT OUT drive the
    -- count negative just as readily, and the guard used to miss both.
    IF v_sign < 0 AND v_new_stock < 0 THEN
        RAISE EXCEPTION 'inventory.insufficient-stock' USING ERRCODE = 'P0001';
    END IF;

    v_status := inventory.fn_stock_status(v_new_stock, v_reorder_level);

    -- Insert movement — the stored quantity carries the sign, so the audit trail
    -- reads as what happened without re-deriving it from the type.
    INSERT INTO inventory.inventory_movements (
        tenant_id, product_id, sku_id, movement_type, quantity,
        reference_type, reference_id, unit_cost, notes, created_by
    )
    VALUES (p_tenant_id, p_product_id, v_sku_id, p_movement_type, v_signed_qty,
            p_reference_type, p_reference_id, v_unit_cost, p_notes, p_created_by)
    RETURNING inventory_movements.id INTO v_movement_id;

    -- Update stock with new status
    UPDATE inventory.product_stock ps
    SET current_quantity = v_new_stock,
        status = v_status,
        last_updated_at = CURRENT_TIMESTAMP
    WHERE ps.sku_id = v_sku_id;

    -- Return with explicit casting
    RETURN QUERY SELECT
        v_movement_id,
        v_new_stock,
        CAST('Movement recorded successfully' AS TEXT);
END;
$$;
COMMENT ON FUNCTION inventory.sp_record_movement IS
    'Records an inventory movement against a SKU and updates that SKU''s '
    'quantities. p_quantity is a magnitude: the sign comes from '
    'inventory.fn_movement_direction(p_movement_type, p_direction) and is what '
    'gets stored, so a SALE reduces stock (INV-013 D1). p_unit_cost is required '
    'and > 0 on PURCHASE and PRODUCTION and stored NULL on every other type '
    '(INV-013 D2/D3). p_sku_id is required when the product tracks stock by '
    'variant and defaults to the product SKU otherwise. Takes the stock row FOR '
    'UPDATE so concurrent outbound movements cannot oversell.';
