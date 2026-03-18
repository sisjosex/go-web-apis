-- Remove tenant_id from purchasing tables.
-- Each tenant has its own dedicated database, so tenant_id is redundant.

-- Drop dependent indexes first
DROP INDEX IF EXISTS purchasing.idx_suppliers_tenant;
DROP INDEX IF EXISTS purchasing.idx_purchase_orders_tenant;
DROP INDEX IF EXISTS purchasing.idx_product_batches_tenant;
DROP INDEX IF EXISTS purchasing.idx_request_for_quotes_tenant;

-- Drop composite unique indexes that include tenant_id
DROP INDEX IF EXISTS purchasing.idx_purchase_orders_number;
DROP INDEX IF EXISTS purchasing.idx_product_batches_batch_unique;

-- Drop FK constraints referencing tenancy schema (does not exist in tenant DBs)
ALTER TABLE purchasing.suppliers
    DROP CONSTRAINT IF EXISTS suppliers_tenant_id_fkey;

ALTER TABLE purchasing.purchase_orders
    DROP CONSTRAINT IF EXISTS purchase_orders_tenant_id_fkey;

-- Drop tenant_id columns
ALTER TABLE purchasing.suppliers
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE purchasing.purchase_orders
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE purchasing.product_batches
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE purchasing.request_for_quotes
    DROP COLUMN IF EXISTS tenant_id;

-- Recreate unique indexes without tenant_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_purchase_orders_number
    ON purchasing.purchase_orders(po_number);

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_batches_batch_unique
    ON purchasing.product_batches(product_id, batch_number);

-- ============================================================
-- Recreate stored procedures without p_tenant_id parameter
-- ============================================================

-- sp_create_supplier
CREATE OR REPLACE FUNCTION purchasing.sp_create_supplier(
    p_name VARCHAR,
    p_contact_person VARCHAR,
    p_email VARCHAR,
    p_phone VARCHAR,
    p_address TEXT,
    p_payment_terms INT
)
RETURNS TABLE(
    id UUID,
    name VARCHAR,
    contact_person VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    address TEXT,
    payment_terms INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    INSERT INTO purchasing.suppliers(name, contact_person, email, phone, address, payment_terms)
    VALUES(p_name, p_contact_person, p_email, p_phone, p_address, COALESCE(p_payment_terms, 30))
    RETURNING
        suppliers.id,
        suppliers.name,
        suppliers.contact_person,
        suppliers.email,
        suppliers.phone,
        suppliers.address,
        suppliers.payment_terms,
        suppliers.is_active,
        suppliers.created_at,
        suppliers.updated_at;
END;
$$;

-- sp_update_supplier
CREATE OR REPLACE FUNCTION purchasing.sp_update_supplier(
    p_supplier_id UUID,
    p_name VARCHAR,
    p_contact_person VARCHAR,
    p_email VARCHAR,
    p_phone VARCHAR,
    p_address TEXT,
    p_payment_terms INT,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    name VARCHAR,
    contact_person VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    address TEXT,
    payment_terms INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    UPDATE purchasing.suppliers
    SET
        name = COALESCE(p_name, name),
        contact_person = COALESCE(p_contact_person, contact_person),
        email = COALESCE(p_email, email),
        phone = COALESCE(p_phone, phone),
        address = COALESCE(p_address, address),
        payment_terms = COALESCE(p_payment_terms, payment_terms),
        is_active = COALESCE(p_is_active, is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_supplier_id
    RETURNING
        suppliers.id,
        suppliers.name,
        suppliers.contact_person,
        suppliers.email,
        suppliers.phone,
        suppliers.address,
        suppliers.payment_terms,
        suppliers.is_active,
        suppliers.created_at,
        suppliers.updated_at;
END;
$$;

-- sp_delete_supplier (soft delete)
CREATE OR REPLACE FUNCTION purchasing.sp_delete_supplier(
    p_supplier_id UUID
)
RETURNS TABLE(
    deleted BOOLEAN
) LANGUAGE plpgsql AS $$
BEGIN
    UPDATE purchasing.suppliers
    SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP
    WHERE id = p_supplier_id;
    RETURN QUERY SELECT TRUE;
END;
$$;

-- sp_get_suppliers_count
CREATE OR REPLACE FUNCTION purchasing.sp_get_suppliers_count()
RETURNS TABLE(
    total_count INT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT CAST(COUNT(*) AS INT) FROM purchasing.suppliers
    WHERE is_active = TRUE;
END;
$$;

-- sp_list_suppliers (paginated)
CREATE OR REPLACE FUNCTION purchasing.sp_list_suppliers(
    p_page_size INT,
    p_offset INT
)
RETURNS TABLE(
    id UUID,
    name VARCHAR,
    contact_person VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    address TEXT,
    payment_terms INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
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
    WHERE s.is_active = TRUE
    ORDER BY s.created_at DESC
    LIMIT p_page_size OFFSET p_offset;
END;
$$;

-- sp_create_purchase_order
CREATE OR REPLACE FUNCTION purchasing.sp_create_purchase_order(
    p_supplier_id UUID,
    p_expected_delivery_date DATE,
    p_notes TEXT DEFAULT NULL,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE(
    po_id UUID,
    po_number VARCHAR,
    supplier_id UUID,
    status VARCHAR,
    total_amount DECIMAL,
    expected_delivery_date DATE,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_po_id UUID := gen_random_uuid();
    v_po_number VARCHAR;
    v_sequence INT;
    v_message TEXT := 'Purchase order created successfully';
BEGIN
    -- Validate supplier exists
    IF NOT EXISTS (SELECT 1 FROM purchasing.suppliers WHERE id = p_supplier_id) THEN
        RAISE EXCEPTION 'supplier.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Generate PO number (PO-YYYY-00001)
    v_sequence := COALESCE((SELECT COUNT(*) + 1 FROM purchasing.purchase_orders), 1);
    v_po_number := 'PO-' || TO_CHAR(CURRENT_DATE, 'YYYY') || '-' || LPAD(v_sequence::TEXT, 5, '0');

    INSERT INTO purchasing.purchase_orders(
        id, supplier_id, po_number, status,
        expected_delivery_date, notes, created_by
    )
    VALUES(
        v_po_id, p_supplier_id, v_po_number, 'draft',
        p_expected_delivery_date, p_notes, p_created_by
    );

    RETURN QUERY
    SELECT
        v_po_id,
        v_po_number,
        p_supplier_id,
        'draft'::VARCHAR,
        0::DECIMAL,
        p_expected_delivery_date,
        v_message::TEXT;
END;
$$;

-- sp_list_purchase_orders (paginated)
CREATE OR REPLACE FUNCTION purchasing.sp_list_purchase_orders(
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
            po.id, po.supplier_id, po.po_number, po.status,
            po.order_date, po.expected_delivery_date, po.total_amount,
            po.paid_amount, po.notes, po.created_by, po.created_at, po.updated_at
        FROM purchasing.purchase_orders po
        WHERE po.status = p_status
        ORDER BY po.created_at DESC
        LIMIT p_page_size OFFSET p_offset;
    ELSE
        RETURN QUERY
        SELECT
            po.id, po.supplier_id, po.po_number, po.status,
            po.order_date, po.expected_delivery_date, po.total_amount,
            po.paid_amount, po.notes, po.created_by, po.created_at, po.updated_at
        FROM purchasing.purchase_orders po
        ORDER BY po.created_at DESC
        LIMIT p_page_size OFFSET p_offset;
    END IF;
END;
$$;

-- sp_get_purchase_orders_count
CREATE OR REPLACE FUNCTION purchasing.sp_get_purchase_orders_count(
    p_status VARCHAR
)
RETURNS TABLE(
    total_count INT
) LANGUAGE plpgsql AS $$
BEGIN
    IF p_status IS NOT NULL THEN
        RETURN QUERY
        SELECT CAST(COUNT(*) AS INT)
        FROM purchasing.purchase_orders
        WHERE status = p_status;
    ELSE
        RETURN QUERY
        SELECT CAST(COUNT(*) AS INT)
        FROM purchasing.purchase_orders;
    END IF;
END;
$$;

-- sp_get_pending_payments
CREATE OR REPLACE FUNCTION purchasing.sp_get_pending_payments()
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
        po.id, po.supplier_id, po.po_number, po.status,
        po.order_date, po.expected_delivery_date, po.total_amount,
        po.paid_amount, po.notes, po.created_by, po.created_at, po.updated_at
    FROM purchasing.purchase_orders po
    JOIN purchasing.purchase_order_invoices inv ON po.id = inv.purchase_order_id
    WHERE inv.status != 'paid'
    ORDER BY inv.due_date ASC;
END;
$$;

-- sp_create_product_batch
CREATE OR REPLACE FUNCTION purchasing.sp_create_product_batch(
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
    INSERT INTO purchasing.product_batches(product_id, batch_number, quantity, unit_cost, receipt_date, expiration_date, status)
    VALUES(p_product_id, p_batch_number, p_quantity, p_unit_cost, p_receipt_date, p_expiration_date, COALESCE(p_status, 'received'))
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

-- sp_get_product_batches
CREATE OR REPLACE FUNCTION purchasing.sp_get_product_batches(
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
        pb.id, pb.product_id, pb.batch_number, pb.quantity,
        pb.unit_cost, pb.receipt_date, pb.expiration_date, pb.status,
        pb.created_at, pb.updated_at
    FROM purchasing.product_batches pb
    WHERE pb.product_id = p_product_id AND pb.status != 'expired'
    ORDER BY pb.receipt_date ASC;
END;
$$;

-- sp_get_oldest_batch_for_sale (FIFO)
CREATE OR REPLACE FUNCTION purchasing.sp_get_oldest_batch_for_sale(
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
        pb.id, pb.product_id, pb.batch_number, pb.quantity,
        pb.unit_cost, pb.receipt_date, pb.expiration_date, pb.status,
        pb.created_at, pb.updated_at
    FROM purchasing.product_batches pb
    WHERE pb.product_id = p_product_id
      AND pb.quantity > 0
      AND pb.status IN ('received', 'partial_sold')
    ORDER BY pb.receipt_date ASC
    LIMIT 1;
END;
$$;

-- sp_create_rfq
CREATE OR REPLACE FUNCTION purchasing.sp_create_rfq(
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
    INSERT INTO purchasing.request_for_quotes(rfq_number, status)
    VALUES(p_rfq_number, COALESCE(p_status, 'draft'))
    RETURNING
        request_for_quotes.id,
        request_for_quotes.rfq_number,
        request_for_quotes.status,
        request_for_quotes.created_at,
        request_for_quotes.updated_at;
END;
$$;

-- sp_select_best_rfq_response
CREATE OR REPLACE FUNCTION purchasing.sp_select_best_rfq_response(
    p_rfq_id UUID
)
RETURNS TABLE(
    status_updated BOOLEAN
) LANGUAGE plpgsql AS $$
BEGIN
    UPDATE purchasing.request_for_quotes
    SET status = 'closed'
    WHERE id = p_rfq_id;
    RETURN QUERY SELECT TRUE;
END;
$$;
