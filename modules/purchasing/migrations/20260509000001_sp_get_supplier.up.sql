-- Migration: sp_get_supplier
-- Module: purchasing
-- Description: Stored procedure to retrieve a single supplier by ID

CREATE OR REPLACE FUNCTION purchasing.sp_get_supplier(
    p_supplier_id UUID,
    p_tenant_id   UUID
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
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.suppliers s
        WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'supplier.not-found' USING ERRCODE = 'P0001';
    END IF;

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
    WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id;
END;
$$;

COMMENT ON FUNCTION purchasing.sp_get_supplier(UUID, UUID) IS
    'Retrieves a single supplier by ID scoped to a tenant. Raises supplier.not-found if not found.';
