-- INV-026: a counting session that closes into adjustments. Lines keep what the stock read when they were first counted.
CREATE TABLE inventory.stock_counts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL,
    number            VARCHAR(30) NOT NULL,
    warehouse_id      UUID        NOT NULL REFERENCES inventory.warehouses (id),
    category_id       UUID        REFERENCES inventory.product_categories (id),
    blind             BOOLEAN     NOT NULL DEFAULT FALSE,
    status            VARCHAR(10) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'paused', 'review', 'closed', 'cancelled')),
    adjustment_number VARCHAR(30),
    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at         TIMESTAMPTZ,
    created_by        UUID,
    closed_by         UUID,
    CONSTRAINT stock_counts_tenant_number_key UNIQUE (tenant_id, number)
);
-- One open count per warehouse and category; also what the nav badge counts.
CREATE UNIQUE INDEX idx_stock_counts_one_open
    ON inventory.stock_counts (tenant_id, warehouse_id, COALESCE(category_id, '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE status IN ('open', 'paused', 'review');
CREATE INDEX idx_stock_counts_tenant_started ON inventory.stock_counts (tenant_id, started_at DESC);

CREATE TABLE inventory.stock_count_lines (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    count_id   UUID          NOT NULL REFERENCES inventory.stock_counts (id) ON DELETE CASCADE,
    product_id UUID          NOT NULL REFERENCES inventory.products (id),
    sku_id     UUID          NOT NULL REFERENCES inventory.product_skus (id),
    expected   NUMERIC(14,3),
    counted    NUMERIC(14,3) CHECK (counted >= 0),
    counted_at TIMESTAMPTZ,
    CONSTRAINT stock_count_lines_sku_key UNIQUE (count_id, sku_id)
);
