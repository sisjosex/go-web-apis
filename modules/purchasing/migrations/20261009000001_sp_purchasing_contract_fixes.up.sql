-- Migration: sp_purchasing_contract_fixes
-- Module: purchasing
-- Description: INFRA-014. The purchasing SPs the endpoint tests reached for the first time:
--   * five declared RETURNS TABLE types their tables do not hold (42804 on every call) — they now
--     return the table's own types: TIMESTAMP dates, INT quantities, INT payment terms;
--   * receiving an order marked it received but never added stock — it now records a PURCHASE
--     movement per item through inventory.sp_record_movement, in the same transaction, and answers
--     the receipt row the repository reads;
--   * nothing wrote supplier_rates, so price comparison was always empty — adding an order item
--     records that supplier's latest cost for the product;
--   * the oldest batch answered no row instead of batch.not-found;
--   * pending payments ordered by a column SELECT DISTINCT did not select (an error on every call);
--   * selecting an RFQ response closed any RFQ id, known or not, and marked no response selected.

-- ----------------------------------------------------------------------------------------------
-- Purchase order
-- ----------------------------------------------------------------------------------------------
DROP FUNCTION IF EXISTS purchasing.sp_get_purchase_order(UUID, UUID);

CREATE FUNCTION purchasing.sp_get_purchase_order(
    p_po_id     UUID,
    p_tenant_id UUID
)
RETURNS TABLE(
    id                     UUID,
    supplier_id            UUID,
    po_number              VARCHAR,
    status                 VARCHAR,
    order_date             TIMESTAMP,
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

COMMENT ON FUNCTION purchasing.sp_get_purchase_order(UUID, UUID) IS
    'Retrieves a single purchase order by ID scoped to a tenant. Raises po.not-found if not found.';

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
#variable_conflict use_column
DECLARE
    v_item_id UUID := gen_random_uuid();
    v_supplier_id UUID;
BEGIN
    SELECT po.supplier_id INTO v_supplier_id
    FROM purchasing.purchase_orders po
    WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status IN ('draft', 'approved');

    IF v_supplier_id IS NULL THEN
        RAISE EXCEPTION 'po.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'po.invalid-quantity' USING ERRCODE = 'P0001';
    END IF;

    IF p_unit_cost <= 0 THEN
        RAISE EXCEPTION 'po.invalid-cost' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO purchasing.purchase_order_items(id, purchase_order_id, product_id, quantity, unit_cost)
    VALUES (v_item_id, p_po_id, p_product_id, p_quantity, p_unit_cost);

    UPDATE purchasing.purchase_orders
    SET total_amount = (
            SELECT COALESCE(SUM(poi.line_total), 0)
            FROM purchasing.purchase_order_items poi
            WHERE poi.purchase_order_id = p_po_id
        ),
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    -- The supplier's latest cost for the product: what price comparison ranks.
    INSERT INTO purchasing.supplier_rates (supplier_id, product_id, unit_cost, payment_terms, last_price_date)
    SELECT v_supplier_id, p_product_id, p_unit_cost, s.payment_terms, CURRENT_TIMESTAMP
    FROM purchasing.suppliers s
    WHERE s.id = v_supplier_id
    ON CONFLICT (supplier_id, product_id) DO UPDATE
    SET unit_cost = EXCLUDED.unit_cost,
        last_price_date = EXCLUDED.last_price_date,
        updated_at = CURRENT_TIMESTAMP;

    RETURN QUERY
    SELECT
        v_item_id,
        p_po_id,
        p_product_id,
        p_quantity,
        p_unit_cost,
        (p_quantity * p_unit_cost)::DECIMAL,
        'Item added to purchase order'::TEXT;
END;
$$;

DROP FUNCTION IF EXISTS purchasing.sp_receive_purchase_order_items(UUID, UUID, TIMESTAMP, UUID, TEXT);

CREATE FUNCTION purchasing.sp_receive_purchase_order_items(
    p_tenant_id UUID,
    p_po_id UUID,
    p_receipt_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    p_received_by UUID DEFAULT NULL,
    p_notes TEXT DEFAULT NULL
)
RETURNS TABLE(
    receipt_id UUID,
    po_id UUID,
    receipt_number VARCHAR,
    received_by UUID,
    notes TEXT,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
DECLARE
    v_receipt_id UUID := gen_random_uuid();
    v_receipt_number VARCHAR;
    v_item RECORD;
BEGIN
    -- FOR UPDATE: two receipts of one order cannot both add its stock.
    PERFORM 1 FROM purchasing.purchase_orders po
    WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id AND po.status IN ('approved', 'invoiced')
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'po.cannot-receive' USING ERRCODE = 'P0001';
    END IF;

    v_receipt_number := 'RCP-' || TO_CHAR(p_receipt_date, 'YYYYMMDD') || '-' || UPPER(LEFT(REPLACE(v_receipt_id::TEXT, '-', ''), 8));

    INSERT INTO purchasing.purchase_order_receipts(id, purchase_order_id, receipt_number, receipt_date, received_by, notes)
    VALUES (v_receipt_id, p_po_id, v_receipt_number, p_receipt_date, p_received_by, p_notes);

    FOR v_item IN
        SELECT poi.product_id, poi.quantity - poi.received_quantity AS pending, poi.unit_cost
        FROM purchasing.purchase_order_items poi
        WHERE poi.purchase_order_id = p_po_id AND poi.status != 'received'
    LOOP
        PERFORM inventory.sp_record_movement(
            p_tenant_id      := p_tenant_id,
            p_product_id     := v_item.product_id,
            p_movement_type  := 'PURCHASE',
            p_quantity       := v_item.pending::DECIMAL,
            p_reference_type := 'purchase_order',
            p_reference_id   := p_po_id,
            p_unit_cost      := v_item.unit_cost,
            p_notes          := v_receipt_number,
            p_created_by     := p_received_by
        );
    END LOOP;

    UPDATE purchasing.purchase_order_items poi
    SET received_quantity = poi.quantity,
        status = 'received',
        updated_at = CURRENT_TIMESTAMP
    WHERE poi.purchase_order_id = p_po_id AND poi.status != 'received';

    UPDATE purchasing.purchase_orders
    SET status = 'received',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_po_id AND tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT r.id, r.purchase_order_id, r.receipt_number, r.received_by, r.notes, r.created_at
    FROM purchasing.purchase_order_receipts r
    WHERE r.id = v_receipt_id;
END;
$$;

COMMENT ON FUNCTION purchasing.sp_receive_purchase_order_items(UUID, UUID, TIMESTAMP, UUID, TEXT) IS
    'Receives an approved or invoiced order: one PURCHASE inventory movement per pending item, items '
    'and order marked received, the receipt row answered. Raises po.cannot-receive otherwise.';

-- ----------------------------------------------------------------------------------------------
-- Order lists: the same order_date type; pending payments ordered by the earliest unpaid due date
-- (SELECT DISTINCT could not order by a column it does not select).
-- ----------------------------------------------------------------------------------------------
DROP FUNCTION IF EXISTS purchasing.sp_list_purchase_orders(UUID, VARCHAR, INT, INT);
DROP FUNCTION IF EXISTS purchasing.sp_get_pending_payments(UUID);

CREATE FUNCTION purchasing.sp_list_purchase_orders(
    p_tenant_id UUID,
    p_status VARCHAR,
    p_page_size INT,
    p_offset INT
)
RETURNS TABLE(
    id                     UUID,
    supplier_id            UUID,
    po_number              VARCHAR,
    status                 VARCHAR,
    order_date             TIMESTAMP,
    expected_delivery_date DATE,
    total_amount           DECIMAL,
    paid_amount            DECIMAL,
    notes                  TEXT,
    created_by             UUID,
    created_at             TIMESTAMP,
    updated_at             TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT po.id, po.supplier_id, po.po_number, po.status, po.order_date, po.expected_delivery_date,
           po.total_amount, po.paid_amount, po.notes, po.created_by, po.created_at, po.updated_at
    FROM purchasing.purchase_orders po
    WHERE po.tenant_id = p_tenant_id AND (p_status IS NULL OR po.status = p_status)
    ORDER BY po.created_at DESC
    LIMIT p_page_size OFFSET p_offset;
END;
$$;

CREATE FUNCTION purchasing.sp_get_pending_payments(
    p_tenant_id UUID
)
RETURNS TABLE(
    id                     UUID,
    supplier_id            UUID,
    po_number              VARCHAR,
    status                 VARCHAR,
    order_date             TIMESTAMP,
    expected_delivery_date DATE,
    total_amount           DECIMAL,
    paid_amount            DECIMAL,
    notes                  TEXT,
    created_by             UUID,
    created_at             TIMESTAMP,
    updated_at             TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT po.id, po.supplier_id, po.po_number, po.status, po.order_date, po.expected_delivery_date,
           po.total_amount, po.paid_amount, po.notes, po.created_by, po.created_at, po.updated_at
    FROM purchasing.purchase_orders po
    JOIN (
        SELECT inv.purchase_order_id, MIN(inv.due_date) AS first_due
        FROM purchasing.purchase_order_invoices inv
        WHERE inv.status != 'paid'
        GROUP BY inv.purchase_order_id
    ) owed ON owed.purchase_order_id = po.id
    WHERE po.tenant_id = p_tenant_id
    ORDER BY owed.first_due ASC;
END;
$$;

-- ----------------------------------------------------------------------------------------------
-- Price comparison
-- ----------------------------------------------------------------------------------------------
DROP FUNCTION IF EXISTS purchasing.sp_get_price_comparison(UUID, UUID);

CREATE FUNCTION purchasing.sp_get_price_comparison(
    p_product_id UUID,
    p_tenant_id  UUID
)
RETURNS TABLE(
    product_id      UUID,
    supplier_id     UUID,
    supplier_name   VARCHAR,
    unit_cost       DECIMAL,
    payment_terms   INT,
    last_price_date TIMESTAMP,
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
        AVG(sr.unit_cost) OVER (PARTITION BY sr.product_id)
    FROM purchasing.supplier_rates sr
    JOIN purchasing.suppliers s ON s.id = sr.supplier_id
    WHERE sr.product_id = p_product_id AND s.tenant_id = p_tenant_id
    ORDER BY sr.unit_cost ASC;
END;
$$;

COMMENT ON FUNCTION purchasing.sp_get_price_comparison(UUID, UUID) IS
    'Returns all supplier prices for a product with computed average cost, ordered by unit cost ascending.';

-- ----------------------------------------------------------------------------------------------
-- Product batches
-- ----------------------------------------------------------------------------------------------
DROP FUNCTION IF EXISTS purchasing.sp_create_product_batch(UUID, UUID, VARCHAR, DECIMAL, DECIMAL, DATE, DATE, VARCHAR);
DROP FUNCTION IF EXISTS purchasing.sp_get_product_batches(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_get_oldest_batch_for_sale(UUID, UUID);

CREATE FUNCTION purchasing.sp_create_product_batch(
    p_tenant_id UUID,
    p_product_id UUID,
    p_batch_number VARCHAR,
    p_quantity INT,
    p_unit_cost DECIMAL,
    p_receipt_date TIMESTAMP,
    p_expiration_date TIMESTAMP,
    p_status VARCHAR
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    batch_number VARCHAR,
    quantity INT,
    unit_cost DECIMAL,
    receipt_date TIMESTAMP,
    expiration_date TIMESTAMP,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.product_batches(tenant_id, product_id, batch_number, quantity, unit_cost, receipt_date, expiration_date, status)
    VALUES (p_tenant_id, p_product_id, p_batch_number, p_quantity, p_unit_cost, p_receipt_date, p_expiration_date, COALESCE(NULLIF(p_status, ''), 'received'))
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

CREATE FUNCTION purchasing.sp_get_product_batches(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    batch_number VARCHAR,
    quantity INT,
    unit_cost DECIMAL,
    receipt_date TIMESTAMP,
    expiration_date TIMESTAMP,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT pb.id, pb.product_id, pb.batch_number, pb.quantity, pb.unit_cost, pb.receipt_date,
           pb.expiration_date, pb.status, pb.created_at, pb.updated_at
    FROM purchasing.product_batches pb
    WHERE pb.tenant_id = p_tenant_id AND pb.product_id = p_product_id AND pb.status != 'expired'
    ORDER BY pb.receipt_date ASC;
END;
$$;

CREATE FUNCTION purchasing.sp_get_oldest_batch_for_sale(
    p_tenant_id UUID,
    p_product_id UUID
)
RETURNS TABLE(
    id UUID,
    product_id UUID,
    batch_number VARCHAR,
    quantity INT,
    unit_cost DECIMAL,
    receipt_date TIMESTAMP,
    expiration_date TIMESTAMP,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT pb.id, pb.product_id, pb.batch_number, pb.quantity, pb.unit_cost, pb.receipt_date,
           pb.expiration_date, pb.status, pb.created_at, pb.updated_at
    FROM purchasing.product_batches pb
    WHERE pb.tenant_id = p_tenant_id AND pb.product_id = p_product_id AND pb.quantity > 0
      AND pb.status IN ('received', 'partial_sold')
    ORDER BY pb.receipt_date ASC
    LIMIT 1;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'batch.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$;

-- ----------------------------------------------------------------------------------------------
-- RFQ
-- ----------------------------------------------------------------------------------------------
DROP FUNCTION IF EXISTS purchasing.sp_add_rfq_item(UUID, UUID, UUID, DECIMAL, TEXT);
DROP FUNCTION IF EXISTS purchasing.sp_get_rfq_items(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_add_rfq_response(UUID, UUID, UUID, DECIMAL, INT, VARCHAR, TEXT);
DROP FUNCTION IF EXISTS purchasing.sp_get_rfq_responses(UUID, UUID);
DROP FUNCTION IF EXISTS purchasing.sp_select_best_rfq_response(UUID, UUID);

CREATE FUNCTION purchasing.sp_add_rfq_item(
    p_tenant_id UUID,
    p_rfq_id UUID,
    p_product_id UUID,
    p_quantity INT,
    p_description TEXT
)
RETURNS TABLE(
    id UUID,
    rfq_id UUID,
    product_id UUID,
    quantity INT,
    description TEXT,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.rfq_items(rfq_id, product_id, quantity, description)
    VALUES (p_rfq_id, p_product_id, p_quantity, p_description)
    RETURNING rfq_items.id, rfq_items.rfq_id, rfq_items.product_id, rfq_items.quantity,
              rfq_items.description, rfq_items.created_at;
END;
$$;

CREATE FUNCTION purchasing.sp_get_rfq_items(
    p_tenant_id UUID,
    p_rfq_id UUID
)
RETURNS TABLE(
    id UUID,
    rfq_id UUID,
    product_id UUID,
    quantity INT,
    description TEXT,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT ri.id, ri.rfq_id, ri.product_id, ri.quantity, ri.description, ri.created_at
    FROM purchasing.rfq_items ri
    WHERE ri.rfq_id = p_rfq_id
    ORDER BY ri.created_at ASC;
END;
$$;

CREATE FUNCTION purchasing.sp_add_rfq_response(
    p_tenant_id UUID,
    p_rfq_id UUID,
    p_supplier_id UUID,
    p_total_price DECIMAL,
    p_delivery_days INT,
    p_payment_terms INT,
    p_notes TEXT
)
RETURNS TABLE(
    id UUID,
    rfq_id UUID,
    supplier_id UUID,
    total_price DECIMAL,
    delivery_days INT,
    payment_terms INT,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM purchasing.suppliers s WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'supplier.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.rfq_responses(rfq_id, supplier_id, total_price, delivery_days, payment_terms, notes)
    VALUES (p_rfq_id, p_supplier_id, p_total_price, p_delivery_days, COALESCE(p_payment_terms, 30), p_notes)
    RETURNING rfq_responses.id, rfq_responses.rfq_id, rfq_responses.supplier_id, rfq_responses.total_price,
              rfq_responses.delivery_days, rfq_responses.payment_terms, rfq_responses.notes,
              rfq_responses.created_at, rfq_responses.updated_at;
END;
$$;

CREATE FUNCTION purchasing.sp_get_rfq_responses(
    p_tenant_id UUID,
    p_rfq_id UUID
)
RETURNS TABLE(
    id UUID,
    rfq_id UUID,
    supplier_id UUID,
    total_price DECIMAL,
    delivery_days INT,
    payment_terms INT,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT rr.id, rr.rfq_id, rr.supplier_id, rr.total_price, rr.delivery_days, rr.payment_terms,
           rr.notes, rr.created_at, rr.updated_at
    FROM purchasing.rfq_responses rr
    WHERE rr.rfq_id = p_rfq_id
    ORDER BY rr.total_price ASC;
END;
$$;

CREATE FUNCTION purchasing.sp_select_best_rfq_response(
    p_tenant_id UUID,
    p_rfq_id UUID,
    p_response_id UUID
)
RETURNS TABLE(
    status_updated BOOLEAN
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM purchasing.request_for_quotes r WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'rfq.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM purchasing.rfq_responses rr WHERE rr.id = p_response_id AND rr.rfq_id = p_rfq_id) THEN
        RAISE EXCEPTION 'rfq.response.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE purchasing.rfq_responses rr
    SET selected = (rr.id = p_response_id),
        updated_at = CURRENT_TIMESTAMP
    WHERE rr.rfq_id = p_rfq_id;

    UPDATE purchasing.request_for_quotes r
    SET status = 'closed',
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_rfq_id AND r.tenant_id = p_tenant_id;

    RETURN QUERY SELECT TRUE;
END;
$$;
