-- TRACK-076 down: every calendar is a schedule calendar again.

DROP INDEX tracking.uq_calendars_organization_scope;
DROP INDEX tracking.uq_calendars_tenant_scope;
ALTER TABLE tracking.calendars DROP COLUMN scope;
