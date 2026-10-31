-- USERS-015 D1: every existing business gets the editable "Operador" role a new one is born with. A
-- business that already has a role of that name keeps it untouched.
WITH created AS (
    INSERT INTO tenancy.roles (tenant_id, name, description)
    SELECT t.id, 'Operador', 'El día a día de taypi go'
    FROM tenancy.tenants t
    ON CONFLICT (tenant_id, name) DO NOTHING
    RETURNING id
)
INSERT INTO tenancy.role_permissions (role_id, permission_code)
SELECT created.id, codes.code
FROM created
CROSS JOIN (VALUES
        ('tracking:companies:read'), ('tracking:companies:write'), ('tracking:companies:delete'),
        ('tracking:vehicles:read'), ('tracking:vehicles:write'), ('tracking:vehicles:delete'),
        ('tracking:routes:read'), ('tracking:routes:write'), ('tracking:routes:delete'),
        ('tracking:riders:read'), ('tracking:riders:write'), ('tracking:riders:delete'),
        ('tracking:assignments:read'), ('tracking:assignments:manage'),
        ('tracking:events:write'), ('tracking:alerts:write'),
        ('tracking:organizations:read'), ('tracking:organizations:write'), ('tracking:organizations:delete'),
        ('tracking:drivers:write'), ('tracking:drivers:delete'),
        ('tracking:documents:write'), ('tracking:reports:read')
) AS codes(code);
