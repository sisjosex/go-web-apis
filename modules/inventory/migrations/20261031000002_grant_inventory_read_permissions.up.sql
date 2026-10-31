-- INV-018 D4: the GETs now check a read permission, so every role that can already write a resource
-- keeps reading it. Owner and admin always pass. A dedicated tenant's database has no tenancy schema.
DO $$
BEGIN
    IF to_regclass('tenancy.role_permissions') IS NULL THEN
        RETURN;
    END IF;

    INSERT INTO tenancy.role_permissions (role_id, permission_code)
    SELECT DISTINCT rp.role_id, grants.read_code
    FROM tenancy.role_permissions rp
    INNER JOIN (VALUES
        ('inventory:products:write',    'inventory:products:read'),
        ('inventory:products:delete',   'inventory:products:read'),
        ('inventory:categories:manage', 'inventory:categories:read'),
        ('inventory:stock:adjust',      'inventory:stock:read'),
        ('inventory:stock:reserve',     'inventory:stock:read'),
        ('inventory:batches:manage',    'inventory:batches:read')
    ) AS grants(write_code, read_code) ON grants.write_code = rp.permission_code
    ON CONFLICT DO NOTHING;
END;
$$;
