-- Tenant modules: tracks which system modules each tenant has enabled
CREATE TABLE IF NOT EXISTS tenancy.tenant_modules (
    id          UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    module_code VARCHAR(100) NOT NULL,
    is_enabled  BOOLEAN NOT NULL DEFAULT TRUE,
    config      JSONB NOT NULL DEFAULT '{}',
    enabled_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    enabled_by  UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_module UNIQUE (tenant_id, module_code)
);

CREATE INDEX IF NOT EXISTS idx_tenant_modules_tenant_id ON tenancy.tenant_modules (tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_modules_module_code ON tenancy.tenant_modules (module_code);
CREATE INDEX IF NOT EXISTS idx_tenant_modules_is_enabled ON tenancy.tenant_modules (is_enabled);

CREATE OR REPLACE FUNCTION tenancy.update_tenant_module_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_tenant_modules_updated_at ON tenancy.tenant_modules;
CREATE TRIGGER trigger_tenant_modules_updated_at
BEFORE UPDATE ON tenancy.tenant_modules
FOR EACH ROW EXECUTE FUNCTION tenancy.update_tenant_module_updated_at();
