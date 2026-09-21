-- Test Fixtures for Tracking Module
-- These are shared test data that can be reused across all tracking tests

DELETE FROM tracking.trips;
DELETE FROM tracking.rider_route_assignments;
DELETE FROM tracking.route_exceptions;
DELETE FROM tracking.route_schedules;
DELETE FROM tracking.calendar_dates;
DELETE FROM tracking.calendars;
DELETE FROM tracking.route_version_stops;
DELETE FROM tracking.route_versions;
DELETE FROM tracking.compliance_documents;
DELETE FROM tracking.document_types;
DELETE FROM tracking.routes;
DELETE FROM tracking.drivers;
DELETE FROM tracking.riders;
DELETE FROM tracking.organization_members;
DELETE FROM tracking.organizations;
DELETE FROM tracking.stop_places;
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
    id, company_id, route_name, origin_address, destination_address, direction, scheduled_start_time, scheduled_end_time, status, created_at, updated_at
) VALUES
(
    '11111111-1111-1111-1111-111111111111'::uuid,
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid,
    'Morning Route',
    'Central Station',
    'Downtown Terminal',
    'outbound',
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
    'inbound',
    '12:00:00'::time,
    '20:00:00'::time,
    'active',
    NOW(),
    NOW()
);

-- ================================================================
-- STOP PLACES — one row per place, shared by every route that calls there (TRACK-007 D1).
-- The ids are the ones route_stops used to carry, so the assignments below are unchanged.
-- ================================================================
INSERT INTO tracking.stop_places (
    id, tenant_id, organization_id, name, address, location, created_at, updated_at
) VALUES
(
    '33333333-3333-3333-3333-333333333333'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,
    NULL,
    'Central Station',
    '1 Central Plaza',
    ST_SetSRID(ST_MakePoint(-74.0060, 40.7128), 4326)::geography,
    NOW(),
    NOW()
),
(
    '44444444-4444-4444-4444-444444444444'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,
    '99999999-9999-9999-9999-999999999999'::uuid,
    'School A',
    '10 School Avenue',
    ST_SetSRID(ST_MakePoint(-73.9851, 40.7589), 4326)::geography,
    NOW(),
    NOW()
),
(
    '55555555-5555-5555-5555-555555555555'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,
    NULL,
    'School B',
    '20 School Road',
    ST_SetSRID(ST_MakePoint(-73.9776, 40.7614), 4326)::geography,
    NOW(),
    NOW()
),
(
    '66666666-6666-6666-6666-666666666666'::uuid,
    '00000000-0000-0000-0000-000000000001'::uuid,
    NULL,
    'Downtown Terminal',
    '5 Downtown Way',
    ST_SetSRID(ST_MakePoint(-73.9840, 40.7549), 4326)::geography,
    NOW(),
    NOW()
);

-- ================================================================
-- ROUTE VERSIONS — one open-ended version per route, in force since 2026-01-01.
-- Both routes call at Central Station and at School B: the same two rows, not copies.
-- ================================================================
INSERT INTO tracking.route_versions (
    id, route_id, effective_from, effective_to, created_by, created_at, updated_at
) VALUES
(
    'a0000000-0000-0000-0000-000000000001'::uuid,
    '11111111-1111-1111-1111-111111111111'::uuid,
    DATE '2026-01-01',
    NULL,
    NULL,
    NOW(),
    NOW()
),
(
    'a0000000-0000-0000-0000-000000000002'::uuid,
    '22222222-2222-2222-2222-222222222222'::uuid,
    DATE '2026-01-01',
    NULL,
    NULL,
    NOW(),
    NOW()
);

INSERT INTO tracking.route_version_stops (
    version_id, stop_place_id, sequence, planned_offset_min, dwell_sec, created_at, updated_at
) VALUES
('a0000000-0000-0000-0000-000000000001'::uuid, '33333333-3333-3333-3333-333333333333'::uuid, 1, 0,  60, NOW(), NOW()),
('a0000000-0000-0000-0000-000000000001'::uuid, '44444444-4444-4444-4444-444444444444'::uuid, 2, 45, 60, NOW(), NOW()),
('a0000000-0000-0000-0000-000000000001'::uuid, '55555555-5555-5555-5555-555555555555'::uuid, 3, 90, 60, NOW(), NOW()),
('a0000000-0000-0000-0000-000000000001'::uuid, '66666666-6666-6666-6666-666666666666'::uuid, 4, 120, 60, NOW(), NOW()),
('a0000000-0000-0000-0000-000000000002'::uuid, '33333333-3333-3333-3333-333333333333'::uuid, 1, 0,  60, NOW(), NOW()),
('a0000000-0000-0000-0000-000000000002'::uuid, '55555555-5555-5555-5555-555555555555'::uuid, 2, 45, 60, NOW(), NOW());

-- ================================================================
-- ROUTE SCHEDULES — open-ended, weekdays only (TRACK-018 D1).
-- 31 is bits 0..4: Monday through Friday. Each route keeps the start time its row already
-- carried, so the seeded plan says the same thing the route row does. The Afternoon Route also
-- runs back at 16:00: an outbound and an inbound trip of one route on one day (TRACK-008).
-- ================================================================
INSERT INTO tracking.route_schedules (
    id, route_id, days_of_week, start_time, valid_from, valid_until, calendar_id, created_at, updated_at
) VALUES
(
    'b0000000-0000-0000-0000-000000000001'::uuid,
    '11111111-1111-1111-1111-111111111111'::uuid,
    31,
    '07:00:00'::time,
    DATE '2026-01-01',
    NULL,
    NULL,
    NOW(),
    NOW()
),
(
    'b0000000-0000-0000-0000-000000000002'::uuid,
    '22222222-2222-2222-2222-222222222222'::uuid,
    31,
    '12:00:00'::time,
    DATE '2026-01-01',
    NULL,
    NULL,
    NOW(),
    NOW()
),
(
    'b0000000-0000-0000-0000-000000000003'::uuid,
    '22222222-2222-2222-2222-222222222222'::uuid,
    31,
    '16:00:00'::time,
    DATE '2026-01-01',
    NULL,
    NULL,
    NOW(),
    NOW()
);

-- ================================================================
-- RIDER ASSIGNMENTS
-- ================================================================
-- Every day from the start of the year, open-ended: the shape TRACK-009's backfill gives an old row.
INSERT INTO tracking.rider_route_assignments (
    id, rider_id, route_id, days_of_week, pickup_stop_place_id, dropoff_stop_place_id,
    valid_from, valid_until, created_at, updated_at
) VALUES
(
    '77777777-7777-7777-7777-777777777777'::uuid,
    'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'::uuid,
    '11111111-1111-1111-1111-111111111111'::uuid,
    127,
    '33333333-3333-3333-3333-333333333333'::uuid,
    '44444444-4444-4444-4444-444444444444'::uuid,
    DATE '2026-01-01',
    NULL,
    NOW(),
    NOW()
),
(
    '88888888-8888-8888-8888-888888888888'::uuid,
    'ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid,
    '22222222-2222-2222-2222-222222222222'::uuid,
    127,
    '33333333-3333-3333-3333-333333333333'::uuid,
    '55555555-5555-5555-5555-555555555555'::uuid,
    DATE '2026-01-01',
    NULL,
    NOW(),
    NOW()
);

-- ================================================================
-- VEHICLE LOCATIONS
-- ================================================================
INSERT INTO tracking.vehicle_locations (vehicle_id, latitude, longitude, speed, heading, recorded_at)
VALUES ('cccccccc-cccc-cccc-cccc-cccccccccccc'::uuid, 40.7128, -74.0060, 45.5, 180.0, NOW());
