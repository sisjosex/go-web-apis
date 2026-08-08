-- Runtime-configurable settings, grouped by module and section so a key reads
-- as <module>.<section>.<key> (e.g. import.images.max_zip_mb). Values are read
-- once at boot; the matching <MODULE>_<SECTION>_<KEY> env var is the fallback.
CREATE SCHEMA IF NOT EXISTS core;

CREATE TABLE IF NOT EXISTS core.settings (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    module      VARCHAR(50)  NOT NULL,
    section     VARCHAR(50)  NOT NULL,
    key         VARCHAR(100) NOT NULL,
    value       TEXT         NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_settings_module_section_key UNIQUE (module, section, key)
);

COMMENT ON TABLE core.settings IS
'Configuration values grouped by module/section, loaded at boot; env vars are the fallback.';
