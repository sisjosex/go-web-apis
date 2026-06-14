CREATE TABLE IF NOT EXISTS tenancy.role_permissions (
    role_id         UUID         NOT NULL REFERENCES tenancy.roles(id) ON DELETE CASCADE,
    permission_code VARCHAR(200) NOT NULL,
    PRIMARY KEY (role_id, permission_code)
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON tenancy.role_permissions(role_id);
