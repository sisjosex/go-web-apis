-- TRACK-005 step 1: organizations are the schools and employers riders belong to, and the entity
-- B2B visibility hangs off (ARCH-001 N1). They live in the tenant database like every other tracking
-- table, so `tenant_id` is the scope and there is no cross-database read anywhere.
--
-- `kind` is the only thing that tells a school from an employer; everything else is the same entity,
-- so one table with a CHECK beats three. `timezone` is an IANA name (`America/Lima`), stored per
-- organization because a tenant may run schools in more than one: a pickup window is read in the
-- organization's own time, never the server's.
--
-- Members are plain tenant users (D3): `organization_members` says which users see an organization
-- and with what role. Which channel a user may sign in through — scoped web, mobile portal — is the
-- access level TRACK-015's middleware reads, not a column here.

CREATE TABLE tracking.organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    kind VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL,
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_organization_kind CHECK (kind IN ('school', 'company', 'other'))
);

-- The list is read alphabetically within a tenant and filtered by kind and is_active; this index
-- serves filter + order so a page is not a sort of the whole table. The trigram index serves the
-- '%term%' search, which no b-tree can.
CREATE INDEX idx_organizations_tenant_name ON tracking.organizations (tenant_id, name, id);
CREATE INDEX idx_organizations_name_trgm ON tracking.organizations USING GIN (name gin_trgm_ops);

CREATE TABLE tracking.organization_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES tracking.organizations(id) ON DELETE CASCADE,
    -- No FK: auth.users lives in the main database for the platform server and beside us in a tenant
    -- database. The membership SPs resolve the identity through auth.users when it is there.
    user_id UUID NOT NULL,
    role VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_organization_member UNIQUE (organization_id, user_id),
    CONSTRAINT chk_organization_member_role CHECK (role IN ('admin', 'viewer', 'supervisor'))
);

CREATE INDEX idx_organization_members_user_id ON tracking.organization_members(user_id);

COMMENT ON TABLE tracking.organizations IS
'Schools and employers whose people ride, scoped to one tenant; kind tells them apart and timezone is the IANA zone their schedules are read in';

COMMENT ON TABLE tracking.organization_members IS
'Which tenant users see an organization and with what role (TRACK-005 D3); the channel they may sign in through is TRACK-015''s access level, not a column here';
