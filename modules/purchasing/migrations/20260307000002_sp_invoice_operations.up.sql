-- Migration: sp_invoice_operations
-- Module: purchasing
-- Description: Stored procedures for purchase order invoice operations

-- ADD INVOICE
CREATE OR REPLACE FUNCTION purchasing.sp_add_invoice(
    p_purchase_order_id UUID,
    p_invoice_number VARCHAR,
    p_invoice_date DATE,
    p_invoice_amount DECIMAL,
    p_tax_amount DECIMAL,
    p_due_date DATE
)
RETURNS TABLE(
    id UUID,
    purchase_order_id UUID,
    invoice_number VARCHAR,
    invoice_date DATE,
    invoice_amount DECIMAL,
    tax_amount DECIMAL,
    due_date DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    INSERT INTO purchasing.purchase_order_invoices(purchase_order_id, invoice_number, invoice_date, invoice_amount, tax_amount, due_date)
    VALUES(p_purchase_order_id, p_invoice_number, p_invoice_date, p_invoice_amount, COALESCE(p_tax_amount, 0), p_due_date)
    RETURNING 
        purchase_order_invoices.id,
        purchase_order_invoices.purchase_order_id,
        purchase_order_invoices.invoice_number,
        purchase_order_invoices.invoice_date,
        purchase_order_invoices.invoice_amount,
        purchase_order_invoices.tax_amount,
        purchase_order_invoices.due_date,
        purchase_order_invoices.status,
        purchase_order_invoices.created_at,
        purchase_order_invoices.updated_at;
END;
$$;

-- MARK INVOICE AS PAID
CREATE OR REPLACE FUNCTION purchasing.sp_mark_invoice_as_paid(
    p_invoice_id UUID
)
RETURNS TABLE(
    id UUID,
    purchase_order_id UUID,
    invoice_number VARCHAR,
    invoice_date DATE,
    invoice_amount DECIMAL,
    tax_amount DECIMAL,
    due_date DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    UPDATE purchasing.purchase_order_invoices
    SET status = 'paid', updated_at = CURRENT_TIMESTAMP
    WHERE id = p_invoice_id
    RETURNING 
        purchase_order_invoices.id,
        purchase_order_invoices.purchase_order_id,
        purchase_order_invoices.invoice_number,
        purchase_order_invoices.invoice_date,
        purchase_order_invoices.invoice_amount,
        purchase_order_invoices.tax_amount,
        purchase_order_invoices.due_date,
        purchase_order_invoices.status,
        purchase_order_invoices.created_at,
        purchase_order_invoices.updated_at;
END;
$$;

-- GET INVOICES FOR PO
CREATE OR REPLACE FUNCTION purchasing.sp_get_invoices(
    p_purchase_order_id UUID
)
RETURNS TABLE(
    id UUID,
    purchase_order_id UUID,
    invoice_number VARCHAR,
    invoice_date DATE,
    invoice_amount DECIMAL,
    tax_amount DECIMAL,
    due_date DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        inv.id,
        inv.purchase_order_id,
        inv.invoice_number,
        inv.invoice_date,
        inv.invoice_amount,
        inv.tax_amount,
        inv.due_date,
        inv.status,
        inv.created_at,
        inv.updated_at
    FROM purchasing.purchase_order_invoices inv
    WHERE inv.purchase_order_id = p_purchase_order_id
    ORDER BY inv.invoice_date DESC;
END;
$$;
