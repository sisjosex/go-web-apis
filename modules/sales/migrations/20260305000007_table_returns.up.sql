-- Table to track customer returns
CREATE TABLE sales.returns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES sales.sales_orders(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES sales.customers(id),
    return_number VARCHAR(50) NOT NULL UNIQUE,
    total_amount DECIMAL(12,2) NOT NULL,
    reason VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'completed')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_returns_order_id ON sales.returns(order_id);
CREATE INDEX idx_returns_customer_id ON sales.returns(customer_id);
CREATE INDEX idx_returns_status ON sales.returns(status);

