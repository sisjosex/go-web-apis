CREATE OR REPLACE FUNCTION inventory.sp_get_product_stock(
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    current_quantity DECIMAL,
    last_updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        ps.id,
        ps.product_id,
        ps.current_quantity,
        ps.last_updated_at
    FROM inventory.product_stock ps
    WHERE ps.product_id = p_product_id;
END;
$$;
