-- INV-020: the ledger without a product filter, newest first, reads this index instead of sorting.
CREATE INDEX idx_inventory_movements_tenant_created ON inventory.inventory_movements (tenant_id, created_at DESC, id DESC);
