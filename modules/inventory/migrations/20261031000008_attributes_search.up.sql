-- INV-024: shared attributes, their values on products and SKUs, what a category brings, the categories' look in the
-- POS, and an indexed search. Values are always a list of strings, so one containment answers every filter (D1).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE inventory.attributes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID         NOT NULL,
    code       VARCHAR(80)  NOT NULL,
    name       VARCHAR(80)  NOT NULL,
    type       VARCHAR(10)  NOT NULL DEFAULT 'list' CHECK (type IN ('list', 'color', 'measure', 'number', 'text')),
    unit       VARCHAR(12),
    filterable BOOLEAN      NOT NULL DEFAULT TRUE,
    -- Talla makes combinations; Marca is a fact about the product. Only list and colour types can be axes.
    axis       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT attributes_tenant_code_key UNIQUE (tenant_id, code)
);
COMMENT ON TABLE inventory.attributes IS 'What describes a product across the catalogue; code is the name without case or accents (INV-024 D1, D2).';

CREATE TABLE inventory.attribute_options (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attribute_id UUID         NOT NULL REFERENCES inventory.attributes (id) ON DELETE CASCADE,
    value        VARCHAR(80)  NOT NULL,
    swatch       VARCHAR(20),
    sort_order   INT          NOT NULL DEFAULT 0,
    CONSTRAINT attribute_options_value_key UNIQUE (attribute_id, value)
);

CREATE TABLE inventory.category_attributes (
    category_id  UUID NOT NULL REFERENCES inventory.product_categories (id) ON DELETE CASCADE,
    attribute_id UUID NOT NULL REFERENCES inventory.attributes (id) ON DELETE CASCADE,
    PRIMARY KEY (category_id, attribute_id)
);
CREATE INDEX idx_category_attributes_attribute ON inventory.category_attributes (attribute_id);
COMMENT ON TABLE inventory.category_attributes IS 'The attributes a category brings to its products, inherited by its subcategories (INV-024 D3).';

ALTER TABLE inventory.products
    ADD COLUMN attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN search_text TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_products_attributes ON inventory.products USING GIN (attributes jsonb_path_ops);
CREATE INDEX idx_products_search_text ON inventory.products USING GIN (search_text gin_trgm_ops);

ALTER TABLE inventory.product_skus ADD COLUMN attributes JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE inventory.product_categories
    ADD COLUMN icon VARCHAR(40),
    ADD COLUMN color VARCHAR(20),
    ADD COLUMN pos_tab BOOLEAN NOT NULL DEFAULT TRUE;

-- D2: every axis points to a shared attribute. Extras (no stock of their own) stay free.
ALTER TABLE inventory.product_variant_groups ADD COLUMN attribute_id UUID REFERENCES inventory.attributes (id);
CREATE INDEX idx_variant_groups_attribute ON inventory.product_variant_groups (attribute_id);

-- A product that survived the purge keeps working: its axes become attributes, by name.
INSERT INTO inventory.attributes (tenant_id, code, name)
SELECT DISTINCT ON (p.tenant_id, regexp_replace(lower(translate(vg.group_type, 'ÁÉÍÓÚÜÑáéíóúüñ', 'AEIOUUNaeiouun')), '[^a-z0-9]+', '-', 'g'))
       p.tenant_id, regexp_replace(lower(translate(vg.group_type, 'ÁÉÍÓÚÜÑáéíóúüñ', 'AEIOUUNaeiouun')), '[^a-z0-9]+', '-', 'g'), vg.group_type
FROM inventory.product_variant_groups vg
INNER JOIN inventory.products p ON p.id = vg.product_id
WHERE vg.affects_inventory;

UPDATE inventory.product_variant_groups vg
SET attribute_id = a.id
FROM inventory.products p, inventory.attributes a
WHERE p.id = vg.product_id AND vg.affects_inventory AND a.tenant_id = p.tenant_id
  AND a.code = regexp_replace(lower(translate(vg.group_type, 'ÁÉÍÓÚÜÑáéíóúüñ', 'AEIOUUNaeiouun')), '[^a-z0-9]+', '-', 'g');

INSERT INTO inventory.attribute_options (attribute_id, value)
SELECT DISTINCT vg.attribute_id, vo.option_name
FROM inventory.product_variant_groups vg
INNER JOIN inventory.product_variant_options vo ON vo.variant_group_id = vg.id
WHERE vg.attribute_id IS NOT NULL;

ALTER TABLE inventory.product_variant_groups
    ADD CONSTRAINT chk_variant_groups_axis_attribute CHECK (NOT affects_inventory OR attribute_id IS NOT NULL);

UPDATE inventory.products SET search_text = lower(translate(name || ' ' || sku, 'ÁÉÍÓÚÜÑáéíóúüñ', 'AEIOUUNaeiouun'));
