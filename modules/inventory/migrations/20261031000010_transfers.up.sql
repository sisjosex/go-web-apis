-- INV-025 D3: a transfer is prepared (reserved at origin), in transit (left origin), received (entered destination).
CREATE TABLE inventory.transfers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL,
    number            VARCHAR(30) NOT NULL,
    status            VARCHAR(12) NOT NULL DEFAULT 'prepared' CHECK (status IN ('prepared', 'in_transit', 'received', 'cancelled')),
    from_warehouse_id UUID        NOT NULL REFERENCES inventory.warehouses (id),
    to_warehouse_id   UUID        NOT NULL REFERENCES inventory.warehouses (id),
    notes             TEXT,
    carrier_note      TEXT,
    prepared_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    prepared_by       UUID,
    shipped_at        TIMESTAMPTZ,
    shipped_by        UUID,
    received_at       TIMESTAMPTZ,
    received_by       UUID,
    CONSTRAINT transfers_tenant_number_key UNIQUE (tenant_id, number),
    CONSTRAINT chk_transfers_two_warehouses CHECK (from_warehouse_id <> to_warehouse_id)
);
CREATE INDEX idx_transfers_tenant_status ON inventory.transfers (tenant_id, status, prepared_at DESC);

CREATE TABLE inventory.transfer_lines (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transfer_id       UUID          NOT NULL REFERENCES inventory.transfers (id) ON DELETE CASCADE,
    product_id        UUID          NOT NULL REFERENCES inventory.products (id),
    sku_id            UUID          NOT NULL REFERENCES inventory.product_skus (id),
    quantity          NUMERIC(14,3) NOT NULL CHECK (quantity > 0),
    received_quantity NUMERIC(14,3),
    CONSTRAINT transfer_lines_sku_key UNIQUE (transfer_id, sku_id)
);
