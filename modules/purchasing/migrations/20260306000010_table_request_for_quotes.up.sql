-- Requests for Quote (RFQ) - for comparing supplier quotes
CREATE TABLE IF NOT EXISTS purchasing.request_for_quotes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    rfq_number VARCHAR(100) NOT NULL,
    status VARCHAR(50) DEFAULT 'draft',    -- draft, sent, responded, closed
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- RFQ line items
CREATE TABLE IF NOT EXISTS purchasing.rfq_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rfq_id UUID NOT NULL REFERENCES purchasing.request_for_quotes(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    quantity INT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- RFQ responses from suppliers
CREATE TABLE IF NOT EXISTS purchasing.rfq_responses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rfq_id UUID NOT NULL REFERENCES purchasing.request_for_quotes(id) ON DELETE CASCADE,
    supplier_id UUID NOT NULL REFERENCES purchasing.suppliers(id) ON DELETE CASCADE,
    total_price DECIMAL(12,2) NOT NULL,
    delivery_days INT,
    payment_terms INT DEFAULT 30,
    notes TEXT,
    selected BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_request_for_quotes_tenant ON purchasing.request_for_quotes(tenant_id);
CREATE UNIQUE INDEX idx_rfq_number ON purchasing.request_for_quotes(tenant_id, rfq_number);
CREATE INDEX idx_request_for_quotes_status ON purchasing.request_for_quotes(status);
CREATE INDEX idx_rfq_items_rfq ON purchasing.rfq_items(rfq_id);
CREATE INDEX idx_rfq_items_product ON purchasing.rfq_items(product_id);
CREATE INDEX idx_rfq_responses_rfq ON purchasing.rfq_responses(rfq_id);
CREATE INDEX idx_rfq_responses_supplier ON purchasing.rfq_responses(supplier_id);
CREATE INDEX idx_rfq_responses_price ON purchasing.rfq_responses(total_price);
