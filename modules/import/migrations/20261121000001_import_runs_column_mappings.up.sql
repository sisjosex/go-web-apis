-- IMPORT-002: what the import engine keeps between files.
--
-- runs is one row per confirmed import, written by the engine itself so every
-- resource has a history without its descriptor doing anything (the page's
-- "Historial de importaciones" and "Importaciones recientes").
--
-- column_mappings remembers how a file's headings map to a resource's fields,
-- per business and per file shape (D3): the signature is the sha256 of the
-- folded headings, sorted, so the same supplier list maps itself on any
-- computer. No foreign keys: a tenant database carries neither tenancy nor
-- every module, and both tables are read by tenant_id first.

CREATE SCHEMA IF NOT EXISTS import;

CREATE TABLE import.runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    resource     VARCHAR(64) NOT NULL,
    file_name    VARCHAR(255),
    total        INTEGER NOT NULL DEFAULT 0,
    created      INTEGER NOT NULL DEFAULT 0,
    updated      INTEGER NOT NULL DEFAULT 0,
    skipped      INTEGER NOT NULL DEFAULT 0,
    failed       INTEGER NOT NULL DEFAULT 0,
    warnings     INTEGER NOT NULL DEFAULT 0,
    performed_by UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_import_runs_tenant_created ON import.runs (tenant_id, created_at DESC);
CREATE INDEX idx_import_runs_tenant_resource ON import.runs (tenant_id, resource, created_at DESC);

CREATE TABLE import.column_mappings (
    tenant_id        UUID NOT NULL,
    resource         VARCHAR(64) NOT NULL,
    header_signature CHAR(64) NOT NULL,
    mapping          JSONB NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, resource, header_signature)
);
