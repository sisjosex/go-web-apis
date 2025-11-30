-- Tenant users relationship table (which users belong to which tenants)
CREATE TABLE IF NOT EXISTS tenancy.tenant_users (
    id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member', -- owner, admin, member, viewer
    is_active BOOLEAN DEFAULT TRUE,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, user_id)
);

-- Indexes for tenant_users table
CREATE INDEX IF NOT EXISTS idx_tenant_users_tenant_id ON tenancy.tenant_users (tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_users_user_id ON tenancy.tenant_users (user_id);
CREATE INDEX IF NOT EXISTS idx_tenant_users_role ON tenancy.tenant_users (role);
CREATE INDEX IF NOT EXISTS idx_tenant_users_is_active ON tenancy.tenant_users (is_active);
CREATE INDEX IF NOT EXISTS idx_tenant_users_joined_at ON tenancy.tenant_users (joined_at);
