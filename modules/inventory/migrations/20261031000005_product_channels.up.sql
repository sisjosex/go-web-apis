-- INV-019 D3: where a product is sold. The POS grid (POS-002) and the online catalogue (SHOP-001) read these
-- with no other query.
ALTER TABLE inventory.products
    ADD COLUMN pos_visible BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN orders_visible BOOLEAN NOT NULL DEFAULT TRUE;
