ALTER TABLE tracking.organization_members
    DROP CONSTRAINT IF EXISTS chk_organization_member_groups,
    DROP COLUMN IF EXISTS groups;
