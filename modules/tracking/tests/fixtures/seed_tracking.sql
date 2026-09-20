-- Test Fixtures for Tracking Module
-- These are shared test data that can be reused across all tracking tests

DELETE FROM tracking.rider_assignments;
DELETE FROM tracking.route_stops;
DELETE FROM tracking.compliance_documents;
DELETE FROM tracking.document_types;
DELETE FROM tracking.routes;
DELETE FROM tracking.drivers;
DELETE FROM tracking.riders;
DELETE FROM tracking.organization_members;
DELETE FROM tracking.organizations;
DELETE FROM tracking.vehicles;
DELETE FROM tracking.transport_companies;

-- ================================================================
-- COMPANIES
-- ================================================================
INSERT INTO tracking.transport_companies (
    id, tenant_id, name, email, phone, address, city, country, registration_number, status, created_at, updated_at
) VALUES
(
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,
    'Main Test Transit',
    'main@test.local',
    '+1234567890',
    '123 Main Street',
    'Test City',
    'Test Country',
    'REG-TEST-001',
    'active',
    NOW(),
    NOW()
),
(
    'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,
    'Secondary Test Transit',
    'secondary@test.local',
    '+0987654321',
    '456 Secondary Ave',
    'Test City',
    'Test Country',
    'REG-TEST-002',
    'active',
    NOW(),
    NOW()
);

-- ================================================================
-- ORGANIZATIONS — the schools and employers riders belong to (TRACK-005)
-- ================================================================
INSERT INTO tracking.organizations (
    id, tenant_id, kind, name, timezone, is_active, created_at, updated_at
) VALUES
(
    '99999999-9999-9999-9999-999999999999'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,
    'school',
    'Main Test School',
    'America/Lima',
    true,
    NOW(),
    NOW()
),
(
    'a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,
    'company',
    'Empty Test Employer',
    'UTC',
    true,
    NOW(),
    NOW()
);

-- orguser@test.local belongs to the main school and to nothing else: that one row is the whole
-- scope TRACK-015's SPs resolve, so the empty employer beside it is what "out of scope" looks like.
INSERT INTO tracking.organization_members (organization_id, user_id, role)
SELECT '99999999-9999-9999-9999-999999999999'::uuid, u.id, 'admin'
FROM auth.users u
WHERE u.email = 'orguser@test.local';

-- ================================================================
-- VEHICLES
-- ================================================================
INSERT INTO tracking.vehicles (
    id, company_id, plate_number, vehicle_type, brand, model, year, capacity, gps_device_id, status, created_at, updated_at
) VALUES
(
    'cccccccc-cccc-cccc-cccc-cccccccccccc'::uuid,
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid,
    'TST-BUS-001',
    'bus',
    'Mercedes',
    'Sprinter',
    2023,
    50,
    'GPS-BUS-001',
    'active',
    NOW(),
    NOW()
),
(
    'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid,
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid,
    'TST-VAN-001',
    'van',
    'Ford',
    'Transit',
    2022,
    20,
    'GPS-VAN-001',
    'active',
    NOW(),
    NOW()
);

-- ================================================================
-- RIDERS
-- ================================================================
INSERT INTO tracking.riders (
    id, organization_id, rider_type, first_name, last_name, email, phone, identification_number, status, created_at, updated_at
) VALUES
(
    'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'::uuid,
    '99999999-9999-9999-9999-999999999999'::uuid,
    'student',
    'John',
    'Student',
    'john.student@test.local',
    '+1111111111',
    'ID-001',
    'active',
    NOW(),
    NOW()
),
(
    'ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid,
    '99999999-9999-9999-9999-999999999999'::uuid,
    'employee',
    'Jane',
    'Employee',
    'jane.employee@test.local',
    '+2222222222',
    'ID-002',
    'active',
    NOW(),
    NOW()
);

-- portal@test.local is John's guardian and no one else's: that one contact row is the whole scope
-- TRACK-017's SPs resolve, so Jane beside him is what "out of scope" looks like for a guardian.
INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, email, user_id, is_primary)
SELECT 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'::uuid,
       'guardian',
       'Portal Guardian',
       '+3333333333',
       u.email,
       u.id,
       true
FROM auth.users u
WHERE u.email = 'portal@test.local';

-- ================================================================
-- ROUTES
-- ================================================================
INSERT INTO tracking.routes (
    id, company_id, route_name, origin_address, destination_address, scheduled_start_time, scheduled_end_time, status, created_at, updated_at
) VALUES
(
    '11111111-1111-1111-1111-111111111111'::uuid,
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid,
    'Morning Route',
    'Central Station',
    'Downtown Terminal',
    '07:00:00'::time,
    '17:00:00'::time,
    'active',
    NOW(),
    NOW()
),
(
    '22222222-2222-2222-2222-222222222222'::uuid,
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid,
    'Afternoon Route',
    'Central Station',
    'Downtown Terminal',
    '12:00:00'::time,
    '20:00:00'::time,
    'active',
    NOW(),
    NOW()
);

-- ================================================================
-- ROUTE STOPS
-- ================================================================
INSERT INTO tracking.route_stops (
    id, route_id, stop_order, location_name, latitude, longitude, estimated_arrival, status, created_at, updated_at
) VALUES
(
    '33333333-3333-3333-3333-333333333333'::uuid,
    '11111111-1111-1111-1111-111111111111'::uuid,
    1,
    'Central Station',
    40.7128,
    -74.0060,
    '07:30:00'::time,
    'active',
    NOW(),
    NOW()
),
(
    '44444444-4444-4444-4444-444444444444'::uuid,
    '11111111-1111-1111-1111-111111111111'::uuid,
    2,
    'School A',
    40.7589,
    -73.9851,
    '08:15:00'::time,
    'active',
    NOW(),
    NOW()
),
(
    '55555555-5555-5555-5555-555555555555'::uuid,
    '11111111-1111-1111-1111-111111111111'::uuid,
    3,
    'School B',
    40.7614,
    -73.9776,
    '09:00:00'::time,
    'active',
    NOW(),
    NOW()
),
(
    '66666666-6666-6666-6666-666666666666'::uuid,
    '11111111-1111-1111-1111-111111111111'::uuid,
    4,
    'Downtown Terminal',
    40.7549,
    -73.9840,
    '17:30:00'::time,
    'active',
    NOW(),
    NOW()
),
(
    '77777777-7777-7777-7777-888888888888'::uuid,
    '22222222-2222-2222-2222-222222222222'::uuid,
    1,
    'Central Station',
    40.7128,
    -74.0060,
    '12:30:00'::time,
    'active',
    NOW(),
    NOW()
),
(
    '88888888-8888-8888-8888-999999999999'::uuid,
    '22222222-2222-2222-2222-222222222222'::uuid,
    2,
    'School B',
    40.7614,
    -73.9776,
    '13:15:00'::time,
    'active',
    NOW(),
    NOW()
);

-- ================================================================
-- RIDER ASSIGNMENTS
-- ================================================================
INSERT INTO tracking.rider_assignments (
    id, rider_id, route_id, pickup_stop_id, dropoff_stop_id, status, created_at, updated_at
) VALUES
(
    '77777777-7777-7777-7777-777777777777'::uuid,
    'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'::uuid,
    '11111111-1111-1111-1111-111111111111'::uuid,
    '33333333-3333-3333-3333-333333333333'::uuid,
    '44444444-4444-4444-4444-444444444444'::uuid,
    'active',
    NOW(),
    NOW()
),
(
    '88888888-8888-8888-8888-888888888888'::uuid,
    'ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid,
    '22222222-2222-2222-2222-222222222222'::uuid,
    '77777777-7777-7777-7777-888888888888'::uuid,
    '88888888-8888-8888-8888-999999999999'::uuid,
    'active',
    NOW(),
    NOW()
);

-- ================================================================
-- VEHICLE LOCATIONS
-- ================================================================
INSERT INTO tracking.vehicle_locations (vehicle_id, latitude, longitude, speed, heading, recorded_at)
VALUES ('cccccccc-cccc-cccc-cccc-cccccccccccc'::uuid, 40.7128, -74.0060, 45.5, 180.0, NOW());
