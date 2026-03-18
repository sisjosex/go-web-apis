-- Reverse: re-add tenant_id columns (NOTE: data cannot be restored)
ALTER TABLE sales.customers ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE sales.sales_orders ADD COLUMN IF NOT EXISTS tenant_id UUID;
