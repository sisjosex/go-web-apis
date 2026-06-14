CREATE TABLE IF NOT EXISTS tenancy.user_roles (
    user_id     UUID        NOT NULL,
    role_id     UUID        NOT NULL REFERENCES tenancy.roles(id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    assigned_by UUID        NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_tenant ON tenancy.user_roles(user_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id     ON tenancy.user_roles(role_id);
