-- Migration: sp_rfq_operations
-- Module: purchasing
-- Description: Stored procedures for Request for Quote (RFQ) operations

-- CREATE RFQ
CREATE OR REPLACE FUNCTION purchasing.sp_create_rfq(
    p_tenant_id UUID,
    p_rfq_number VARCHAR,
    p_status VARCHAR
)
RETURNS TABLE(
    id UUID,
    rfq_number VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    INSERT INTO purchasing.request_for_quotes(tenant_id, rfq_number, status)
    VALUES(p_tenant_id, p_rfq_number, COALESCE(p_status, 'draft'))
    RETURNING
        request_for_quotes.id,
        request_for_quotes.rfq_number,
        request_for_quotes.status,
        request_for_quotes.created_at,
        request_for_quotes.updated_at;
END;
$$;

-- ADD RFQ ITEM
CREATE OR REPLACE FUNCTION purchasing.sp_add_rfq_item(
    p_tenant_id UUID,
    p_rfq_id UUID,
    p_product_id UUID,
    p_quantity DECIMAL,
    p_description TEXT
)
RETURNS TABLE(
    id UUID,
    rfq_id UUID,
    product_id UUID,
    quantity DECIMAL,
    description TEXT,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    -- Validate RFQ belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.rfq_items(rfq_id, product_id, quantity, description)
    VALUES(p_rfq_id, p_product_id, p_quantity, p_description)
    RETURNING
        rfq_items.id,
        rfq_items.rfq_id,
        rfq_items.product_id,
        rfq_items.quantity,
        rfq_items.description,
        rfq_items.created_at;
END;
$$;

-- ADD RFQ RESPONSE
CREATE OR REPLACE FUNCTION purchasing.sp_add_rfq_response(
    p_tenant_id UUID,
    p_rfq_id UUID,
    p_supplier_id UUID,
    p_total_price DECIMAL,
    p_delivery_days INT,
    p_payment_terms VARCHAR,
    p_notes TEXT
)
RETURNS TABLE(
    id UUID,
    rfq_id UUID,
    supplier_id UUID,
    total_price DECIMAL,
    delivery_days INT,
    payment_terms VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    -- Validate RFQ belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate supplier belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.suppliers s WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'supplier.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.rfq_responses(rfq_id, supplier_id, total_price, delivery_days, payment_terms, notes)
    VALUES(p_rfq_id, p_supplier_id, p_total_price, p_delivery_days, p_payment_terms, p_notes)
    RETURNING
        rfq_responses.id,
        rfq_responses.rfq_id,
        rfq_responses.supplier_id,
        rfq_responses.total_price,
        rfq_responses.delivery_days,
        rfq_responses.payment_terms,
        rfq_responses.notes,
        rfq_responses.created_at,
        rfq_responses.updated_at;
END;
$$;

-- GET RFQ ITEMS
CREATE OR REPLACE FUNCTION purchasing.sp_get_rfq_items(
    p_tenant_id UUID,
    p_rfq_id UUID
)
RETURNS TABLE(
    id UUID,
    rfq_id UUID,
    product_id UUID,
    quantity DECIMAL,
    description TEXT,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    -- Validate RFQ belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        ri.id,
        ri.rfq_id,
        ri.product_id,
        ri.quantity,
        ri.description,
        ri.created_at
    FROM purchasing.rfq_items ri
    WHERE ri.rfq_id = p_rfq_id
    ORDER BY ri.created_at ASC;
END;
$$;

-- GET RFQ RESPONSES
CREATE OR REPLACE FUNCTION purchasing.sp_get_rfq_responses(
    p_tenant_id UUID,
    p_rfq_id UUID
)
RETURNS TABLE(
    id UUID,
    rfq_id UUID,
    supplier_id UUID,
    total_price DECIMAL,
    delivery_days INT,
    payment_terms VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    -- Validate RFQ belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rr.id,
        rr.rfq_id,
        rr.supplier_id,
        rr.total_price,
        rr.delivery_days,
        rr.payment_terms,
        rr.notes,
        rr.created_at,
        rr.updated_at
    FROM purchasing.rfq_responses rr
    WHERE rr.rfq_id = p_rfq_id
    ORDER BY rr.total_price ASC;
END;
$$;

-- SELECT BEST RFQ RESPONSE (Close RFQ)
CREATE OR REPLACE FUNCTION purchasing.sp_select_best_rfq_response(
    p_tenant_id UUID,
    p_rfq_id UUID
)
RETURNS TABLE(
    status_updated BOOLEAN
) LANGUAGE plpgsql AS $$
BEGIN
    UPDATE purchasing.request_for_quotes r
    SET status = 'closed'
    WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id;

    RETURN QUERY SELECT TRUE;
END;
$$;
