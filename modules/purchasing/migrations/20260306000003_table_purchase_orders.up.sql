-- Purchase Orders table
CREATE TABLE IF NOT EXISTS purchasing.purchase_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    supplier_id UUID NOT NULL REFERENCES purchasing.suppliers(id) ON DELETE RESTRICT,
    po_number VARCHAR(50) NOT NULL,          -- PO-2026-00001
    status VARCHAR(20) DEFAULT 'draft',      -- draft, approved, received, invoiced, paid, cancelled
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expected_delivery_date DATE,
    total_amount DECIMAL(12,2) DEFAULT 0,
    paid_amount DECIMAL(12,2) DEFAULT 0,
    notes TEXT,
    created_by UUID REFERENCES auth.users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_purchase_orders_tenant ON purchasing.purchase_orders(tenant_id);
CREATE INDEX idx_purchase_orders_supplier ON purchasing.purchase_orders(supplier_id);
CREATE INDEX idx_purchase_orders_status ON purchasing.purchase_orders(status);
CREATE UNIQUE INDEX idx_purchase_orders_number ON purchasing.purchase_orders(tenant_id, po_number);
