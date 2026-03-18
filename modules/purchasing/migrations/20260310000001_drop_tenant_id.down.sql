-- Reverse: re-add tenant_id columns (NOTE: data cannot be restored)
ALTER TABLE purchasing.suppliers ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE purchasing.purchase_orders ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE purchasing.product_batches ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE purchasing.request_for_quotes ADD COLUMN IF NOT EXISTS tenant_id UUID;
