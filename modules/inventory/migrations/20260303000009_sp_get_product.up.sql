CREATE OR REPLACE FUNCTION inventory.sp_get_product(
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
    sku VARCHAR,
    name VARCHAR,
    description TEXT,
    base_price DECIMAL,
    has_variants BOOLEAN,
    status VARCHAR,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        p.id,
        p.sku,
        p.name,
        p.description,
        p.base_price,
        p.has_variants,
        p.status,
        p.created_at
    FROM inventory.products p
    WHERE p.id = p_product_id AND p.status = 'active';
END;
$$;
