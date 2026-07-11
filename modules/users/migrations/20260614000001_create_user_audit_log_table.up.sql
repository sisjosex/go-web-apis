CREATE TABLE IF NOT EXISTS users.user_audit_log (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id       UUID        NOT NULL REFERENCES tenancy.tenants(id),
    action          VARCHAR(30) NOT NULL,
    target_user_id  UUID        REFERENCES auth.users(id) ON DELETE SET NULL,
    performed_by    UUID        REFERENCES auth.users(id) ON DELETE SET NULL,
    metadata        JSONB,
    source          VARCHAR(10) NOT NULL DEFAULT 'user',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ual_tenant_id      ON users.user_audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_ual_created_at     ON users.user_audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_ual_target_user_id ON users.user_audit_log(target_user_id);
CREATE INDEX IF NOT EXISTS idx_ual_performed_by   ON users.user_audit_log(performed_by);

COMMENT ON TABLE users.user_audit_log IS 'Audit trail for user management operations within a tenant';
