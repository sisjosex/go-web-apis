-- Create sales orders table
CREATE TABLE sales.sales_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    customer_id UUID NOT NULL REFERENCES sales.customers(id) ON DELETE RESTRICT,
    order_number VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled')),
    sub_total DECIMAL(12, 2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(12, 2) NOT NULL DEFAULT 0,
    total DECIMAL(12, 2) NOT NULL DEFAULT 0,
    discount_amount DECIMAL(12, 2) DEFAULT 0,
    shipping_address VARCHAR(500),
    notes TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for sales orders
CREATE UNIQUE INDEX idx_sales_orders_order_number ON sales.sales_orders(tenant_id, order_number);
CREATE INDEX idx_sales_orders_tenant_id ON sales.sales_orders(tenant_id);
CREATE INDEX idx_sales_orders_customer_id ON sales.sales_orders(customer_id);
CREATE INDEX idx_sales_orders_status ON sales.sales_orders(status);
CREATE INDEX idx_sales_orders_created_at ON sales.sales_orders(created_at);

-- Create order update timestamp trigger
CREATE OR REPLACE FUNCTION sales.sales_orders_update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER sales_orders_update_timestamp
BEFORE UPDATE ON sales.sales_orders
FOR EACH ROW
EXECUTE FUNCTION sales.sales_orders_update_timestamp();
