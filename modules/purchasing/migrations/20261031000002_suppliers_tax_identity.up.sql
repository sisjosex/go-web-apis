-- PURCH-001: who a supplier is for the purchase book, as a customer (SALES-001 D2).
ALTER TABLE purchasing.suppliers
    ADD COLUMN tax_id VARCHAR(30),
    ADD COLUMN legal_name VARCHAR(255);
CREATE INDEX idx_suppliers_tenant_active_name ON purchasing.suppliers (tenant_id, is_active, name);
