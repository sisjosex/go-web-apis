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
    'member',
    true
FROM auth.users u
WHERE u.email = 'admin@test.local';
