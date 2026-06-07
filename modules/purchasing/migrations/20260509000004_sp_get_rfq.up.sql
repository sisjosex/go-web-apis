-- Migration: sp_get_rfq
-- Module: purchasing
-- Description: Stored procedure to retrieve a single RFQ by ID

CREATE OR REPLACE FUNCTION purchasing.sp_get_rfq(
    p_tenant_id UUID,
    p_rfq_id    UUID
)
RETURNS TABLE(
    id         UUID,
    rfq_number VARCHAR,
    status     VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.request_for_quotes r
        WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.rfq_number,
        r.status,
        r.created_at,
        r.updated_at
    FROM purchasing.request_for_quotes r
    WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id;
END;
$$;

COMMENT ON FUNCTION purchasing.sp_get_rfq(UUID, UUID) IS
    'Retrieves a single RFQ by ID scoped to a tenant. Raises rfq.not-found if not found.';
