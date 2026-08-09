-- Reverses 20260810000001: drops the filtered 5-arg / 9-column signature and recreates
-- the original 3-arg / 8-column sp_list_products verbatim. Dropping alone would leave
-- GET /inventory/products 500ing.

DROP FUNCTION IF EXISTS inventory.sp_list_products(UUID, INT, INT, UUID, VARCHAR);

CREATE FUNCTION inventory.sp_list_products(
    p_tenant_id UUID,
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
    WHERE p.tenant_id = p_tenant_id AND p.status = 'active'
    ORDER BY p.created_at DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$;

COMMENT ON FUNCTION inventory.sp_list_products IS 'Lists active products for a tenant';
