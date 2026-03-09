-- Table to track which batches are assigned to which order items (for FIFO fulfillment)
CREATE TABLE sales.order_batch_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES sales.sales_orders(id) ON DELETE CASCADE,
    order_item_id UUID NOT NULL REFERENCES sales.order_items(id) ON DELETE CASCADE,
    product_batch_id UUID NOT NULL REFERENCES inventory.product_batches(id),
    quantity_assigned DECIMAL(10,2) NOT NULL CHECK (quantity_assigned > 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Index for finding batches for an order
    UNIQUE(order_item_id, product_batch_id)
);

CREATE INDEX idx_order_batch_assignments_order_id ON sales.order_batch_assignments(order_id);
CREATE INDEX idx_order_batch_assignments_batch_id ON sales.order_batch_assignments(product_batch_id);
