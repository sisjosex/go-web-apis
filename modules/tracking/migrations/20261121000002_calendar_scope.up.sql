-- TRACK-076 D1: a calendar reaches routes three ways. A schedule calendar is read by the schedules that
-- point at it (as before); the one tenant calendar by every route; an organization calendar by every
-- route of that client. The month page writes the last two without naming a schedule.

ALTER TABLE tracking.calendars
    ADD COLUMN scope VARCHAR(12) NOT NULL DEFAULT 'schedule'
    CONSTRAINT chk_calendar_scope CHECK (scope IN ('schedule', 'tenant', 'organization'));

COMMENT ON COLUMN tracking.calendars.scope IS
'schedule: read by the schedules that point at it · tenant: read by every route · organization: read by every route of organization_id (TRACK-076 D1)';

CREATE UNIQUE INDEX uq_calendars_tenant_scope ON tracking.calendars (tenant_id) WHERE scope = 'tenant';
CREATE UNIQUE INDEX uq_calendars_organization_scope ON tracking.calendars (organization_id) WHERE scope = 'organization';
