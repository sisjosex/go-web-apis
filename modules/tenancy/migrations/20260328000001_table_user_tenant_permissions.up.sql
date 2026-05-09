CREATE TABLE IF NOT EXISTS tenancy.user_tenant_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    permission_code VARCHAR(200) NOT NULL,
    granted_by UUID NOT NULL,
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, tenant_id, permission_code)
);

CREATE INDEX IF NOT EXISTS idx_utp_user_tenant ON tenancy.user_tenant_permissions(user_id, tenant_id);
