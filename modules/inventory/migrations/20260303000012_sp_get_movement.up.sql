CREATE OR REPLACE FUNCTION inventory.sp_get_movement(
    p_tenant_id UUID,
    p_movement_id UUID
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    movement_type VARCHAR,
    quantity DECIMAL,
    reference_type VARCHAR,
    reference_id UUID,
    unit_cost DECIMAL,
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
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
        m.created_at
    FROM inventory.inventory_movements m
    WHERE m.tenant_id = p_tenant_id AND m.id = p_movement_id;
END;
$$;
