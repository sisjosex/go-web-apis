-- Create sales schema
CREATE SCHEMA IF NOT EXISTS sales;

-- Create customers table
CREATE TABLE sales.customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    address VARCHAR(500),
    city VARCHAR(100),
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country VARCHAR(100),
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'blocked')),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for customers
CREATE UNIQUE INDEX idx_customers_email_unique ON sales.customers(email);

-- Create customers audit/update trigger
CREATE OR REPLACE FUNCTION sales.customers_update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER customers_update_timestamp
BEFORE UPDATE ON sales.customers
FOR EACH ROW
EXECUTE FUNCTION sales.customers_update_timestamp();
