-- INV-025 D1: a row of stock per combination and warehouse. Every business has one default warehouse, and whatever
-- names no warehouse (sales, purchases, the POS until a register has one) uses it.
CREATE TABLE inventory.warehouses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL,
    name            VARCHAR(120) NOT NULL,
    address         VARCHAR(200),
    city            VARCHAR(80),
    kind            VARCHAR(10)  NOT NULL DEFAULT 'store' CHECK (kind IN ('store', 'storage', 'vehicle')),
    manager_user_id UUID,
    is_default      BOOLEAN      NOT NULL DEFAULT FALSE,
    status          VARCHAR(10)  NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    sort_order      INT          NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_warehouses_one_default ON inventory.warehouses (tenant_id) WHERE is_default;
CREATE INDEX idx_warehouses_tenant ON inventory.warehouses (tenant_id, status);
COMMENT ON TABLE inventory.warehouses IS 'Where a business keeps its stock; one default per business (INV-025 D1).';

ALTER TABLE inventory.product_stock ADD COLUMN warehouse_id UUID REFERENCES inventory.warehouses (id);
ALTER TABLE inventory.inventory_movements ADD COLUMN warehouse_id UUID REFERENCES inventory.warehouses (id);

-- A business with stock that survived the purge keeps it in its Principal.
INSERT INTO inventory.warehouses (tenant_id, name, is_default)
SELECT DISTINCT p.tenant_id, 'Principal', TRUE
FROM inventory.product_stock ps
INNER JOIN inventory.products p ON p.id = ps.product_id;

UPDATE inventory.product_stock ps
SET warehouse_id = w.id
FROM inventory.products p, inventory.warehouses w
WHERE p.id = ps.product_id AND w.tenant_id = p.tenant_id AND w.is_default;

UPDATE inventory.inventory_movements m
SET warehouse_id = w.id
FROM inventory.warehouses w
WHERE w.tenant_id = m.tenant_id AND w.is_default;

ALTER TABLE inventory.product_stock ALTER COLUMN warehouse_id SET NOT NULL;
ALTER TABLE inventory.product_stock DROP CONSTRAINT product_stock_sku_id_key;
ALTER TABLE inventory.product_stock ADD CONSTRAINT product_stock_sku_warehouse_key UNIQUE (sku_id, warehouse_id);
CREATE INDEX idx_product_stock_warehouse_sku ON inventory.product_stock (warehouse_id, sku_id);
