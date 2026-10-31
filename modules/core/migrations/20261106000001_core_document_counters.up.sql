-- CORE-001: one correlative per tenant and series, gapless: the counter moves inside the document's own
-- transaction, so a rollback gives the number back. year is 0 for a series that never resets.
CREATE TABLE core.document_counters (
    tenant_id  UUID        NOT NULL,
    series     VARCHAR(80) NOT NULL,
    year       INT         NOT NULL DEFAULT 0,
    next_value BIGINT      NOT NULL,
    PRIMARY KEY (tenant_id, series, year)
);

COMMENT ON TABLE core.document_counters IS 'The last number issued per tenant, series and year; written only by core.fn_next_document_number.';
