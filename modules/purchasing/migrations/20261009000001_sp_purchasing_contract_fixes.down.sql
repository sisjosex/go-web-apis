-- Restores the purchasing SPs as they were before INFRA-014's contract fixes.

DROP FUNCTION IF EXISTS purchasing.sp_get_purchase_order(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_receive_purchase_order_items(UUID, UUID, TIMESTAMP, UUID, TEXT);
DROP FUNCTION IF EXISTS purchasing.sp_get_price_comparison(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_create_product_batch(UUID, UUID, VARCHAR, INT, DECIMAL, TIMESTAMP, TIMESTAMP, VARCHAR);
DROP FUNCTION IF EXISTS purchasing.sp_get_product_batches(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_get_oldest_batch_for_sale(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_add_rfq_item(UUID, UUID, UUID, INT, TEXT);
DROP FUNCTION IF EXISTS purchasing.sp_get_rfq_items(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_add_rfq_response(UUID, UUID, UUID, DECIMAL, INT, INT, TEXT);
DROP FUNCTION IF EXISTS purchasing.sp_get_rfq_responses(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_select_best_rfq_response(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_list_purchase_orders(UUID, VARCHAR, INT, INT);
DROP FUNCTION IF EXISTS purchasing.sp_get_pending_payments(UUID);

CREATE OR REPLACE FUNCTION purchasing.sp_get_purchase_order(
    p_po_id     UUID,
    p_tenant_id UUID
)
RETURNS TABLE(
    id                     UUID,
    supplier_id            UUID,
    po_number              VARCHAR,
    status                 VARCHAR,
    order_date             DATE,
    expected_delivery_date DATE,
    total_amount           DECIMAL,
    paid_amount            DECIMAL,
    notes                  TEXT,
    created_by             UUID,
    created_at             TIMESTAMP,
    updated_at             TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.purchase_orders po
        WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'po.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        po.id,
        po.supplier_id,
        po.po_number,
        po.status,
        po.order_date,
        po.expected_delivery_date,
        po.total_amount,
        po.paid_amount,
        po.notes,
        po.created_by,
        po.created_at,
        po.updated_at
    FROM purchasing.purchase_orders po
    WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id;
END;
$$;

CREATE OR REPLACE FUNCTION purchasing.sp_add_purchase_order_item(
    p_tenant_id UUID,
    p_po_id UUID,
    p_product_id UUID,
    p_quantity INT,
    p_unit_cost DECIMAL
)
RETURNS TABLE(
    item_id UUID,
    po_id UUID,
    product_id UUID,
    quantity INT,
    unit_cost DECIMAL,
    line_total DECIMAL,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_item_id UUID := gen_random_uuid();
    v_line_total DECIMAL;
    v_message TEXT := 'Item added to purchase order';
BEGIN
    -- Validate PO exists, belongs to tenant, and is in draft/approved status
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_orders po WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status IN ('draft', 'approved')) THEN
        RAISE EXCEPTION 'po.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    -- Validate product exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate quantity and cost
    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'po.invalid-quantity' USING ERRCODE = 'P0001';
    END IF;

    IF p_unit_cost <= 0 THEN
        RAISE EXCEPTION 'po.invalid-cost' USING ERRCODE = 'P0001';
    END IF;

    v_line_total := p_quantity * p_unit_cost;

    -- Insert item
    INSERT INTO purchasing.purchase_order_items(
        id, purchase_order_id, product_id, quantity, unit_cost
    )
    VALUES(v_item_id, p_po_id, p_product_id, p_quantity, p_unit_cost);

    -- Update PO total
    UPDATE purchasing.purchase_orders
    SET total_amount = (
        SELECT COALESCE(SUM(poi.line_total), 0)
        FROM purchasing.purchase_order_items poi
        WHERE poi.purchase_order_id = p_po_id
    ),
    updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        v_item_id,
        p_po_id,
        p_product_id,
        p_quantity,
        p_unit_cost,
        v_line_total,
        v_message::TEXT;
END;
$$;

CREATE OR REPLACE FUNCTION purchasing.sp_receive_purchase_order_items(
    p_tenant_id UUID,
    p_po_id UUID,
    p_receipt_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    p_received_by UUID DEFAULT NULL,
    p_notes TEXT DEFAULT NULL
)
RETURNS TABLE(
    receipt_id UUID,
    po_id UUID,
    po_status VARCHAR,
    items_received INT,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_receipt_id UUID := gen_random_uuid();
    v_receipt_number VARCHAR;
    v_items_count INT;
    v_message TEXT := 'Goods received and inventory updated';
BEGIN
    -- Validate PO exists, belongs to tenant, and is approved or invoiced
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_orders po WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status IN ('approved', 'invoiced')) THEN
        RAISE EXCEPTION 'po.cannot-receive' USING ERRCODE = 'P0001';
    END IF;

    -- Generate receipt number
    v_receipt_number := 'RCP-' || TO_CHAR(CURRENT_DATE, 'YYYYMMDD') || '-' || LPAD(CAST(EXTRACT(EPOCH FROM p_receipt_date) * 1000 AS BIGINT)::TEXT, 5, '0');

    -- Create receipt
    INSERT INTO purchasing.purchase_order_receipts(
        id, purchase_order_id, receipt_number, receipt_date, received_by, notes
    )
    VALUES(v_receipt_id, p_po_id, v_receipt_number, p_receipt_date, p_received_by, p_notes);

    -- Mark all items as received
    UPDATE purchasing.purchase_order_items poi
    SET received_quantity = poi.quantity,
        status = 'received',
        updated_at = CURRENT_TIMESTAMP
    WHERE poi.purchase_order_id = p_po_id AND poi.status != 'received';

    GET DIAGNOSTICS v_items_count = ROW_COUNT;

    -- Update PO status to received
    UPDATE purchasing.purchase_orders
    SET status = 'received',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        v_receipt_id,
        p_po_id,
        'received'::VARCHAR,
        v_items_count,
        v_message::TEXT;
END;
$$;

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

CREATE OR REPLACE FUNCTION purchasing.sp_get_product_batches(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
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

CREATE OR REPLACE FUNCTION purchasing.sp_get_oldest_batch_for_sale(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
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

CREATE OR REPLACE FUNCTION purchasing.sp_list_purchase_orders(
    p_tenant_id UUID,
    p_status VARCHAR,
    p_page_size INT,
    p_offset INT
)
RETURNS TABLE(
    id UUID,
    supplier_id UUID,
    po_number VARCHAR,
    status VARCHAR,
    order_date DATE,
    expected_delivery_date DATE,
    total_amount DECIMAL,
    paid_amount DECIMAL,
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF p_status IS NOT NULL THEN
        RETURN QUERY
        SELECT
            po.id,
            po.supplier_id,
            po.po_number,
            po.status,
            po.order_date,
            po.expected_delivery_date,
            po.total_amount,
            po.paid_amount,
            po.notes,
            po.created_by,
            po.created_at,
            po.updated_at
        FROM purchasing.purchase_orders po
        WHERE po.tenant_id = p_tenant_id AND po.status = p_status
        ORDER BY po.created_at DESC
        LIMIT p_page_size OFFSET p_offset;
    ELSE
        RETURN QUERY
        SELECT
            po.id,
            po.supplier_id,
            po.po_number,
            po.status,
            po.order_date,
            po.expected_delivery_date,
            po.total_amount,
            po.paid_amount,
            po.notes,
            po.created_by,
            po.created_at,
            po.updated_at
        FROM purchasing.purchase_orders po
        WHERE po.tenant_id = p_tenant_id
        ORDER BY po.created_at DESC
        LIMIT p_page_size OFFSET p_offset;
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION purchasing.sp_get_pending_payments(
    p_tenant_id UUID
)
RETURNS TABLE(
    id UUID,
    supplier_id UUID,
    po_number VARCHAR,
    status VARCHAR,
    order_date DATE,
    expected_delivery_date DATE,
    total_amount DECIMAL,
    paid_amount DECIMAL,
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT DISTINCT
        po.id,
        po.supplier_id,
        po.po_number,
        po.status,
        po.order_date,
        po.expected_delivery_date,
        po.total_amount,
        po.paid_amount,
        po.notes,
        po.created_by,
        po.created_at,
        po.updated_at
    FROM purchasing.purchase_orders po
    JOIN purchasing.purchase_order_invoices inv ON po.id = inv.purchase_order_id
    WHERE po.tenant_id = p_tenant_id AND inv.status != 'paid'
    ORDER BY inv.due_date ASC;
END;
$$;
