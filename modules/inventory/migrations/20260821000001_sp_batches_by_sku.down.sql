-- Restores both signatures as they stood before INV-014: no sku_id, no sku, no
-- combination filter, and sp_list_batches_by_product hiding only 'expired'.

DROP FUNCTION IF EXISTS inventory.sp_list_batches_by_product(UUID, UUID, BOOLEAN, UUID);
CREATE FUNCTION inventory.sp_list_batches_by_product(
    p_tenant_id UUID,
    p_product_id UUID,
    p_only_active BOOLEAN DEFAULT TRUE
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    lot_number VARCHAR,
    purchase_date DATE,
    expiry_date DATE,
    unit_cost DECIMAL,
    initial_quantity DECIMAL,
    current_quantity DECIMAL,
    status VARCHAR,
    days_to_expiry INT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.lot_number,
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        pb.status,
        pb.expiry_date - CURRENT_DATE
    FROM inventory.product_batches pb
    WHERE pb.tenant_id = p_tenant_id
        AND pb.product_id = p_product_id
        AND (NOT p_only_active OR pb.status != 'expired')
    ORDER BY pb.expiry_date ASC, pb.created_at ASC;
END;
$$;

DROP FUNCTION IF EXISTS inventory.sp_get_expiring_batches(UUID, INT);
CREATE FUNCTION inventory.sp_get_expiring_batches(
    p_tenant_id UUID,
    p_warning_days INT
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    lot_number VARCHAR,
    purchase_date DATE,
    expiry_date DATE,
    unit_cost DECIMAL,
    initial_quantity DECIMAL,
    current_quantity DECIMAL,
    status VARCHAR,
    days_to_expiry INT,
    product_name VARCHAR
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        pb.lot_number,
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        pb.status,
        CAST(pb.expiry_date - CURRENT_DATE AS INT),
        CAST(p.name AS VARCHAR)
    FROM inventory.product_batches pb
    INNER JOIN inventory.products p
        ON p.id = pb.product_id
        AND p.tenant_id = pb.tenant_id
    WHERE pb.tenant_id = p_tenant_id
        AND pb.expiry_date - CURRENT_DATE <= p_warning_days
        AND pb.expiry_date - CURRENT_DATE > 0
        AND pb.status IN ('active', 'expiring_soon')
    ORDER BY pb.expiry_date ASC;
END;
$$;

COMMENT ON FUNCTION inventory.sp_get_expiring_batches IS 'Lists batches expiring within the warning window, including the product name';
