-- ================================================================
-- TEST TENANT SEED FILE
-- ================================================================
-- Creates a test tenant and adds seeded users as members.
-- Used by inventory, sales, and purchasing integration tests.
--
-- Tenant slug: test-company
-- Owner: superadmin@test.local
-- Member: admin@test.local

-- Clean up in case re-seeding
DELETE FROM tenancy.tenant_users
WHERE tenant_id IN (SELECT id FROM tenancy.tenants WHERE slug = 'test-company');
DELETE FROM tenancy.tenants WHERE slug = 'test-company';

-- Create tenant (insert directly to avoid SP subscription-limit checks)
INSERT INTO tenancy.tenants (id, slug, name, schema_name, is_active, is_suspended, settings)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'test-company',
    'Test Company',
    'public',
    true,
    false,
    '{}'
);

-- Add super_admin as owner
INSERT INTO tenancy.tenant_users (tenant_id, user_id, role, is_active)
SELECT
    '00000000-0000-0000-0000-000000000001',
    u.id,
    'owner',
    true
FROM auth.users u
WHERE u.email = 'superadmin@test.local';

-- Add admin as member
INSERT INTO tenancy.tenant_users (tenant_id, user_id, role, is_active)
SELECT
    '00000000-0000-0000-0000-000000000001',
    u.id,
    'admin',
    true
FROM auth.users u
WHERE u.email = 'admin@test.local';

-- Add the two TRACK-015 access levels: an organization user scoped by its memberships, and a
-- portal guardian the web tenant middleware refuses.
INSERT INTO tenancy.tenant_users (tenant_id, user_id, role, is_active)
SELECT
    '00000000-0000-0000-0000-000000000001',
    u.id,
    'organization',
    true
FROM auth.users u
WHERE u.email = 'orguser@test.local';

INSERT INTO tenancy.tenant_users (tenant_id, user_id, role, is_active)
SELECT
    '00000000-0000-0000-0000-000000000001',
    u.id,
    'portal',
    true
FROM auth.users u
WHERE u.email = 'portal@test.local';

-- A driver account is mobile-only too (TRACK-006 D1); the web refuses it exactly as it refuses portal.
INSERT INTO tenancy.tenant_users (tenant_id, user_id, role, is_active)
SELECT
    '00000000-0000-0000-0000-000000000001',
    u.id,
    'driver',
    true
FROM auth.users u
WHERE u.email = 'driver@test.local';

-- The organization user's capabilities come from an assigned role, exactly like any other member's
-- (TRACK-015 D1): the access level decides what it sees, the role decides what it may do.
DELETE FROM tenancy.roles
WHERE tenant_id = '00000000-0000-0000-0000-000000000001' AND name = 'Organization Staff';

INSERT INTO tenancy.roles (id, tenant_id, name, description, is_system)
VALUES (
    '00000000-0000-0000-0000-0000000000a1',
    '00000000-0000-0000-0000-000000000001',
    'Organization Staff',
    'Reads and edits the riders of its own organizations',
    false
);

INSERT INTO tenancy.role_permissions (role_id, permission_code)
VALUES
    ('00000000-0000-0000-0000-0000000000a1', 'tracking:riders:read'),
    ('00000000-0000-0000-0000-0000000000a1', 'tracking:riders:write'),
    ('00000000-0000-0000-0000-0000000000a1', 'tracking:riders:delete'),
    ('00000000-0000-0000-0000-0000000000a1', 'tracking:routes:read'),
    ('00000000-0000-0000-0000-0000000000a1', 'tracking:assignments:read')
ON CONFLICT DO NOTHING;

INSERT INTO tenancy.user_roles (user_id, role_id, tenant_id, assigned_by)
SELECT u.id, '00000000-0000-0000-0000-0000000000a1', '00000000-0000-0000-0000-000000000001', u.id
FROM auth.users u
WHERE u.email = 'orguser@test.local'
ON CONFLICT DO NOTHING;

-- Enable all relevant modules for the test tenant
INSERT INTO tenancy.tenant_modules (tenant_id, module_code, is_enabled)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'tracking', true),
    ('00000000-0000-0000-0000-000000000001', 'inventory', true),
    ('00000000-0000-0000-0000-000000000001', 'sales', true),
    ('00000000-0000-0000-0000-000000000001', 'purchasing', true)
ON CONFLICT (tenant_id, module_code) DO NOTHING;
