CREATE OR REPLACE FUNCTION inventory.sp_get_product_stock(
    p_product_id UUID
)
RETURNS TABLE(
    current_quantity DECIMAL,
    reserved_quantity DECIMAL,
    available_quantity DECIMAL,
    reorder_level DECIMAL,
    status VARCHAR,
    last_updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        ps.current_quantity,
        ps.reserved_quantity,
        (ps.current_quantity - ps.reserved_quantity) AS available_quantity,
        ps.reorder_level,
        ps.status,
        ps.last_updated_at
    FROM inventory.product_stock ps
    WHERE ps.product_id = p_product_id;
END;
$$;
