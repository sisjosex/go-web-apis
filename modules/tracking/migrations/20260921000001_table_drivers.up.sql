-- TRACK-006 step 1: drivers are the people who drive a carrier's vehicles. They live in the tenant
-- database beside the rest of tracking, so `tenant_id` is the scope and nothing here reads across
-- databases.
--
-- A driver belongs to a transport company, not to an organization: the carrier employs them, the
-- school or employer only rides with them. Deleting the carrier takes its drivers with it, the same
-- way it already takes its vehicles and routes.
--
-- `user_id` is a free reference to `auth.users`, not an FK: auth.users sits in the main database for
-- the platform server and beside us in a tenant database, exactly as organization_members already
-- assumes. It is the link to a `driver` access-level account (D1) — which level that account holds
-- is tenancy's column, never a column here. UNIQUE per tenant so one account drives for one driver
-- record; NULLs stay distinct, so any number of drivers may have no account yet.
--
-- `license_expires_on` is a DATE, not a timestamp: a licence expires on a calendar day in the
-- driver's own country, and storing an instant would make that day move with the server's zone.

CREATE TABLE tracking.drivers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL REFERENCES tracking.transport_companies(id) ON DELETE CASCADE,
    user_id UUID,
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    license_number VARCHAR(100) NOT NULL,
    license_class VARCHAR(50),
    license_expires_on DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_driver_status CHECK (status IN ('active', 'inactive', 'suspended')),
    CONSTRAINT uk_driver_user UNIQUE (tenant_id, user_id),
    CONSTRAINT uk_driver_license UNIQUE (tenant_id, license_number)
);

-- The list is read by last name inside one company, so filter + order come off this one index and a
-- page is not a sort of the whole table.
CREATE INDEX idx_drivers_company_name ON tracking.drivers (tenant_id, company_id, last_name, first_name, id);

-- A route may name the driver who normally drives it; a per-trip override is TRACK-008. SET NULL
-- rather than CASCADE: losing a driver must not delete the route they drove. The index is what makes
-- both that SET NULL and the "still has routes" check on delete a lookup instead of a table scan.
ALTER TABLE tracking.routes
    ADD COLUMN default_driver_id UUID REFERENCES tracking.drivers(id) ON DELETE SET NULL;

CREATE INDEX idx_routes_default_driver_id ON tracking.routes (default_driver_id);

COMMENT ON TABLE tracking.drivers IS
'The people who drive a carrier''s vehicles, scoped to one tenant; user_id links the driver to a tenant account whose access level is `driver` (TRACK-006 D1), and is free of an FK because auth.users may live in another database';

COMMENT ON COLUMN tracking.routes.default_driver_id IS
'The driver who normally drives this route; NULL when none is set, and set to NULL rather than cascading when that driver is deleted';
