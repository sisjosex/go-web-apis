-- Inventory movements (audit trail)
CREATE TABLE IF NOT EXISTS inventory.inventory_movements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES inventory.products(id),
    movement_type VARCHAR(50) NOT NULL CHECK (movement_type IN ('PURCHASE', 'SALE', 'ADJUSTMENT', 'TRANSFER', 'RETURN', 'WASTE', 'PRODUCTION')),
    quantity DECIMAL(12,2) NOT NULL,
    reference_type VARCHAR(50),
    reference_id UUID,
    unit_cost DECIMAL(12,2),
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_movements_product ON inventory.inventory_movements(product_id);
CREATE INDEX idx_movements_type ON inventory.inventory_movements(movement_type);
CREATE INDEX idx_movements_reference ON inventory.inventory_movements(reference_type, reference_id);
CREATE INDEX idx_movements_created_at ON inventory.inventory_movements(created_at);
