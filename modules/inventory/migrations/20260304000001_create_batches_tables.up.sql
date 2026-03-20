-- Create product_batches table for tracking inventory by lot
CREATE TABLE IF NOT EXISTS inventory.product_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    lot_number VARCHAR(100) NOT NULL,
    purchase_date DATE NOT NULL,
    expiry_date DATE NOT NULL,
    unit_cost DECIMAL(10, 2) NOT NULL,
    initial_quantity DECIMAL(10, 2) NOT NULL,
    current_quantity DECIMAL(10, 2) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, expired, depleted
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast lookup by product_id and expiry date
CREATE INDEX idx_product_batches_tenant_id ON inventory.product_batches(tenant_id);
CREATE INDEX idx_product_batches_product_id ON inventory.product_batches(product_id);
CREATE INDEX idx_product_batches_expiry_date ON inventory.product_batches(expiry_date);
CREATE INDEX idx_product_batches_product_expiry ON inventory.product_batches(product_id, expiry_date);

-- Create batch_movements table to track FIFO consumption
CREATE TABLE IF NOT EXISTS inventory.batch_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    batch_id UUID NOT NULL REFERENCES inventory.product_batches(id) ON DELETE CASCADE,
    movement_id UUID REFERENCES inventory.inventory_movements(id) ON DELETE SET NULL,
    quantity_consumed DECIMAL(10, 2) NOT NULL,
    cost_of_goods_sold DECIMAL(10, 2) NOT NULL,
    movement_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_batch_movements_tenant_id ON inventory.batch_movements(tenant_id);
CREATE INDEX idx_batch_movements_batch_id ON inventory.batch_movements(batch_id);
CREATE INDEX idx_batch_movements_movement_id ON inventory.batch_movements(movement_id);

-- Add batch_id column to inventory_movements if not exists
ALTER TABLE inventory.inventory_movements
ADD COLUMN batch_id UUID REFERENCES inventory.product_batches(id) ON DELETE SET NULL;

CREATE INDEX idx_inventory_movements_batch_id ON inventory.inventory_movements(batch_id);

-- Add cost_of_goods_sold column to sales table (if sales module exists)
-- ALTER TABLE sales.sales
-- ADD COLUMN cost_of_goods_sold DECIMAL(10, 2);
