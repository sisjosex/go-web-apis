-- INV-023: a product sells and buys in presentations of one stock counted in its base unit, and each presentation
-- has its barcodes. D2: a seeded standard list, siat_code empty until invoicing loads the official catalogue.
CREATE TABLE inventory.units (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID,
    code           VARCHAR(20)  NOT NULL,
    name_es        VARCHAR(60)  NOT NULL,
    name_en        VARCHAR(60)  NOT NULL,
    symbol         VARCHAR(12)  NOT NULL,
    siat_code      VARCHAR(20),
    allow_fraction BOOLEAN      NOT NULL DEFAULT FALSE,
    sort_order     INT          NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX idx_units_standard_code ON inventory.units (code) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX idx_units_tenant_code ON inventory.units (tenant_id, code) WHERE tenant_id IS NOT NULL;
COMMENT ON TABLE inventory.units IS 'The units a presentation can be in: a seeded standard list, tenant_id NULL, plus a business''s own.';

INSERT INTO inventory.units (code, name_es, name_en, symbol, allow_fraction, sort_order) VALUES
    ('UND', 'Unidad', 'Unit', 'u.', FALSE, 0),
    ('MTR', 'Metro', 'Metre', 'm', TRUE, 1),
    ('KGM', 'Kilo', 'Kilo', 'kg', TRUE, 2),
    ('LTR', 'Litro', 'Litre', 'L', TRUE, 3),
    ('MTK', 'Metro²', 'Square metre', 'm²', TRUE, 4),
    ('PAR', 'Par', 'Pair', 'par', FALSE, 5),
    ('CAJ', 'Caja', 'Box', 'cj', FALSE, 6),
    ('PAQ', 'Paquete', 'Pack', 'paq', FALSE, 7),
    ('BLI', 'Blíster', 'Blister', 'bl', FALSE, 8),
    ('ROL', 'Rollo', 'Roll', 'rollo', FALSE, 9),
    ('DOC', 'Docena', 'Dozen', 'doc', FALSE, 10),
    ('BOL', 'Bolsa', 'Bag', 'bls', FALSE, 11),
    ('BOT', 'Botella', 'Bottle', 'bot', FALSE, 12),
    ('FRA', 'Frasco', 'Jar', 'fco', FALSE, 13),
    ('LAT', 'Lata', 'Can', 'lata', FALSE, 14),
    ('GAL', 'Galón', 'Gallon', 'gal', TRUE, 15),
    ('GRM', 'Gramo', 'Gram', 'g', TRUE, 16),
    ('MLT', 'Mililitro', 'Millilitre', 'ml', TRUE, 17),
    ('CMT', 'Centímetro', 'Centimetre', 'cm', TRUE, 18),
    ('TON', 'Tonelada', 'Tonne', 't', TRUE, 19),
    ('SAC', 'Saco', 'Sack', 'saco', FALSE, 20),
    ('BAR', 'Barra', 'Bar', 'bar', FALSE, 21),
    ('PZA', 'Pieza', 'Piece', 'pza', FALSE, 22),
    ('JGO', 'Juego', 'Set', 'jgo', FALSE, 23),
    ('TAB', 'Tableta', 'Tablet', 'tab', FALSE, 24),
    ('AMP', 'Ampolla', 'Ampoule', 'amp', FALSE, 25),
    ('SOB', 'Sobre', 'Sachet', 'sob', FALSE, 26),
    ('TUB', 'Tubo', 'Tube', 'tubo', FALSE, 27),
    ('PLA', 'Plancha', 'Sheet', 'pl', FALSE, 28),
    ('ATA', 'Atado', 'Bundle', 'at', FALSE, 29);

CREATE TABLE inventory.product_units (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID          NOT NULL,
    product_id          UUID          NOT NULL REFERENCES inventory.products (id) ON DELETE CASCADE,
    unit_id             UUID          NOT NULL REFERENCES inventory.units (id),
    factor              NUMERIC(14,3) NOT NULL CHECK (factor > 0),
    is_base             BOOLEAN       NOT NULL DEFAULT FALSE,
    allow_fraction      BOOLEAN       NOT NULL DEFAULT FALSE,
    sellable            BOOLEAN       NOT NULL DEFAULT TRUE,
    pos_visible         BOOLEAN       NOT NULL DEFAULT TRUE,
    is_sale_default     BOOLEAN       NOT NULL DEFAULT FALSE,
    is_purchase_default BOOLEAN       NOT NULL DEFAULT FALSE,
    price               NUMERIC(12,2),
    is_active           BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT chk_product_units_base_factor CHECK (NOT is_base OR factor = 1)
);
CREATE UNIQUE INDEX idx_product_units_one_base ON inventory.product_units (product_id) WHERE is_base;
CREATE INDEX idx_product_units_product ON inventory.product_units (product_id);
COMMENT ON TABLE inventory.product_units IS 'How a product sells and buys: factor base units per presentation. Stock, lots and movements stay in the base unit.';

CREATE TABLE inventory.sku_codes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL,
    code            VARCHAR(64) NOT NULL,
    sku_id          UUID        NOT NULL REFERENCES inventory.product_skus (id) ON DELETE CASCADE,
    product_unit_id UUID        NOT NULL REFERENCES inventory.product_units (id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT sku_codes_tenant_code_key UNIQUE (tenant_id, code)
);
COMMENT ON TABLE inventory.sku_codes IS 'The barcodes of a business: each names a combination and a presentation (INV-023 D1).';

-- A product created between the purge (INV-028) and this migration still gets its base presentation.
INSERT INTO inventory.product_units (tenant_id, product_id, unit_id, factor, is_base, is_sale_default, is_purchase_default)
SELECT p.tenant_id, p.id, u.id, 1, TRUE, TRUE, TRUE
FROM inventory.products p
CROSS JOIN inventory.units u
WHERE u.code = 'UND' AND u.tenant_id IS NULL;
