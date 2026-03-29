CREATE TABLE IF NOT EXISTS inventory.product_media (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL,
    product_id        UUID        NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    variant_option_id UUID        REFERENCES inventory.product_variant_options(id) ON DELETE CASCADE,
    media_type        VARCHAR(10) NOT NULL CHECK (media_type IN ('image', 'video')),
    url               TEXT        NOT NULL,
    alt_text          VARCHAR(255),
    is_primary        BOOLEAN     NOT NULL DEFAULT false,
    sort_order        INT         NOT NULL DEFAULT 0,
    created_at        TIMESTAMP   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_product_media_product_id        ON inventory.product_media(product_id);
CREATE INDEX idx_product_media_variant_option_id ON inventory.product_media(variant_option_id);
CREATE INDEX idx_product_media_tenant_id         ON inventory.product_media(tenant_id);
