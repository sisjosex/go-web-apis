-- Migration: sp_price_comparison
-- Module: purchasing
-- Description: Stored procedures for price comparison and best supplier lookup

-- PRICE COMPARISON: all supplier prices for a product with average cost
CREATE OR REPLACE FUNCTION purchasing.sp_get_price_comparison(
    p_product_id UUID,
    p_tenant_id  UUID
)
RETURNS TABLE(
    product_id      UUID,
    supplier_id     UUID,
    supplier_name   VARCHAR,
    unit_cost       DECIMAL,
    payment_terms   INT,
    last_price_date DATE,
    average_cost    DECIMAL
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        sr.product_id,
        sr.supplier_id,
        s.name,
        sr.unit_cost,
        s.payment_terms,
        sr.last_price_date,
        COALESCE(AVG(sr.unit_cost) OVER (PARTITION BY sr.product_id), sr.unit_cost)
    FROM purchasing.supplier_rates sr
    JOIN purchasing.suppliers s ON s.id = sr.supplier_id
    WHERE sr.product_id = p_product_id AND s.tenant_id = p_tenant_id
    ORDER BY sr.unit_cost ASC;
END;
$$;

COMMENT ON FUNCTION purchasing.sp_get_price_comparison(UUID, UUID) IS
    'Returns all supplier prices for a product with computed average cost, ordered by unit cost ascending.';

-- BEST SUPPLIER: supplier with the lowest unit cost for a product
CREATE OR REPLACE FUNCTION purchasing.sp_get_best_supplier_for_product(
    p_product_id UUID,
    p_tenant_id  UUID
)
RETURNS TABLE(
    id             UUID,
    name           VARCHAR,
    contact_person VARCHAR,
    email          VARCHAR,
    phone          VARCHAR,
    address        TEXT,
    payment_terms  INT,
    is_active      BOOLEAN,
    created_at     TIMESTAMP,
    updated_at     TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        s.name,
        s.contact_person,
        s.email,
        s.phone,
        s.address,
        s.payment_terms,
        s.is_active,
        s.created_at,
        s.updated_at
    FROM purchasing.suppliers s
    JOIN purchasing.supplier_rates sr ON s.id = sr.supplier_id
    WHERE sr.product_id = p_product_id AND s.tenant_id = p_tenant_id
    ORDER BY sr.unit_cost ASC
    LIMIT 1;
END;
$$;

COMMENT ON FUNCTION purchasing.sp_get_best_supplier_for_product(UUID, UUID) IS
    'Returns the supplier with the lowest unit cost for a given product, scoped to the tenant.';
