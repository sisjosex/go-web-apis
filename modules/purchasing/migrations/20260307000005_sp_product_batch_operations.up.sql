-- Migration: sp_product_batch_operations
-- Module: purchasing
-- Description: Stored procedures for product batch operations (FIFO tracking)

-- CREATE PRODUCT BATCH
CREATE OR REPLACE FUNCTION purchasing.sp_create_product_batch(
    p_tenant_id UUID,
    p_product_id UUID,
    p_batch_number VARCHAR,
    p_quantity DECIMAL,
    p_unit_cost DECIMAL,
    p_receipt_date DATE,
    p_expiration_date DATE,
    p_status VARCHAR
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    product_id UUID,
    batch_number VARCHAR,
    quantity DECIMAL,
    unit_cost DECIMAL,
    receipt_date DATE,
    expiration_date DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    INSERT INTO purchasing.product_batches(tenant_id, product_id, batch_number, quantity, unit_cost, receipt_date, expiration_date, status)
    VALUES(p_tenant_id, p_product_id, p_batch_number, p_quantity, p_unit_cost, p_receipt_date, p_expiration_date, COALESCE(p_status, 'received'))
    RETURNING 
        product_batches.id,
        product_batches.tenant_id,
        product_batches.product_id,
        product_batches.batch_number,
        product_batches.quantity,
        product_batches.unit_cost,
        product_batches.receipt_date,
        product_batches.expiration_date,
        product_batches.status,
        product_batches.created_at,
        product_batches.updated_at;
END;
$$;

-- GET PRODUCT BATCHES
CREATE OR REPLACE FUNCTION purchasing.sp_get_product_batches(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    product_id UUID,
    batch_number VARCHAR,
    quantity DECIMAL,
    unit_cost DECIMAL,
    receipt_date DATE,
    expiration_date DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        pb.id,
        pb.tenant_id,
        pb.product_id,
        pb.batch_number,
        pb.quantity,
        pb.unit_cost,
        pb.receipt_date,
        pb.expiration_date,
        pb.status,
        pb.created_at,
        pb.updated_at
    FROM purchasing.product_batches pb
    WHERE pb.tenant_id = p_tenant_id AND pb.product_id = p_product_id AND pb.status != 'expired'
    ORDER BY pb.receipt_date ASC;
END;
$$;

-- GET OLDEST BATCH FOR SALE (FIFO principle)
CREATE OR REPLACE FUNCTION purchasing.sp_get_oldest_batch_for_sale(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    product_id UUID,
    batch_number VARCHAR,
    quantity DECIMAL,
    unit_cost DECIMAL,
    receipt_date DATE,
    expiration_date DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        pb.id,
        pb.tenant_id,
        pb.product_id,
        pb.batch_number,
        pb.quantity,
        pb.unit_cost,
        pb.receipt_date,
        pb.expiration_date,
        pb.status,
        pb.created_at,
        pb.updated_at
    FROM purchasing.product_batches pb
    WHERE pb.tenant_id = p_tenant_id AND pb.product_id = p_product_id AND pb.quantity > 0 AND pb.status IN ('received', 'partial_sold')
    ORDER BY pb.receipt_date ASC
    LIMIT 1;
END;
$$;
