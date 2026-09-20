-- The users module also runs against tenant databases, which carry auth.* but
-- no tenancy.*. An inline REFERENCES tenancy.tenants(id) is validated when the
-- table is created, so it made this migration — and with it the whole users
-- module — impossible to apply to a fresh tenant database. The FK is therefore
-- added separately, guarded by to_regclass, exactly as sp_soft_delete_user does.
CREATE TABLE IF NOT EXISTS users.user_audit_log (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id       UUID        NOT NULL,
    action          VARCHAR(30) NOT NULL,
    target_user_id  UUID        REFERENCES auth.users(id) ON DELETE SET NULL,
    performed_by    UUID        REFERENCES auth.users(id) ON DELETE SET NULL,
    metadata        JSONB,
    source          VARCHAR(10) NOT NULL DEFAULT 'user',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
    IF to_regclass('tenancy.tenants') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1 FROM pg_constraint
           WHERE conname = 'user_audit_log_tenant_id_fkey'
             AND conrelid = 'users.user_audit_log'::regclass
       )
    THEN
        ALTER TABLE users.user_audit_log
            ADD CONSTRAINT user_audit_log_tenant_id_fkey
            FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_ual_tenant_id      ON users.user_audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_ual_created_at     ON users.user_audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_ual_target_user_id ON users.user_audit_log(target_user_id);
CREATE INDEX IF NOT EXISTS idx_ual_performed_by   ON users.user_audit_log(performed_by);

COMMENT ON TABLE users.user_audit_log IS 'Audit trail for user management operations within a tenant';
