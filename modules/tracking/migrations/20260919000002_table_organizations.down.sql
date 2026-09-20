-- organization_members goes with its parent (ON DELETE CASCADE is on the FK, not on the DROP).
DROP TABLE IF EXISTS tracking.organization_members;
DROP TABLE IF EXISTS tracking.organizations;
