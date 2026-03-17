-- Get all batches expiring within warning days
DROP FUNCTION IF EXISTS inventory.sp_get_expiring_batches(INT);
CREATE OR REPLACE FUNCTION inventory.sp_get_expiring_batches(
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
        CAST(pb.expiry_date - CURRENT_DATE AS INT)
    FROM inventory.product_batches pb
    WHERE pb.expiry_date - CURRENT_DATE <= p_warning_days
        AND pb.expiry_date - CURRENT_DATE > 0
        AND pb.status IN ('active', 'expiring_soon')
    ORDER BY pb.expiry_date ASC;
END;
$$;
