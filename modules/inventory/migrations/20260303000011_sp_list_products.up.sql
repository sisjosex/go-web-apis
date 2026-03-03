CREATE OR REPLACE FUNCTION inventory.sp_list_products(
    p_limit INT DEFAULT 20,
    p_offset INT DEFAULT 0
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
    WHERE p.status = 'active'
    ORDER BY p.created_at DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$;
