-- Routes (schedules, paths)
CREATE TABLE tracking.routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    start_location VARCHAR(255),
    end_location VARCHAR(255),
    estimated_duration INTERVAL,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_route_company FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id)
);

CREATE INDEX idx_routes_company_id ON tracking.routes(company_id);
