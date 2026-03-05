-- Get batch details by ID
CREATE OR REPLACE FUNCTION inventory.sp_get_batch(
    p_batch_id UUID
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
        CAST(pb.id AS UUID),
        CAST(pb.product_id AS UUID),
        CAST(pb.lot_number AS VARCHAR),
        CAST(pb.purchase_date AS DATE),
        CAST(pb.expiry_date AS DATE),
        CAST(pb.unit_cost AS DECIMAL),
        CAST(pb.initial_quantity AS DECIMAL),
        CAST(pb.current_quantity AS DECIMAL),
        CAST(pb.status AS VARCHAR),
        (pb.expiry_date - CURRENT_DATE)
    FROM inventory.product_batches pb
    WHERE pb.id = p_batch_id;
END;
$$;
