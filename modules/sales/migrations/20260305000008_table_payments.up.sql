-- Table to track order payments
CREATE TABLE sales.payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES sales.sales_orders(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES sales.customers(id),
    amount DECIMAL(12,2) NOT NULL CHECK (amount > 0),
    payment_method VARCHAR(50) NOT NULL CHECK (payment_method IN ('cash', 'card', 'bank_transfer', 'check', 'other')),
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed', 'refunded')),
    reference_number VARCHAR(100),
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_payments_order_id ON sales.payments(order_id);
CREATE INDEX idx_payments_customer_id ON sales.payments(customer_id);
CREATE INDEX idx_payments_status ON sales.payments(status);

