-- TRACK-018 step 1: when a route runs, which days it does not, and what happens differently on one
-- of them. TRACK-007 versioned *what* a route calls at; this versions *when* it goes.
--
-- The recurrence is a weekday bitmask (D1): bit 0 is Monday … bit 6 is Sunday, the same integer in
-- the column and in JSON. The preview is then one generate_series with a bit test and stays inside
-- the SP — an RRULE would have moved the plan into Go and forced the same library on TRACK-008 and
-- on mobile.
--
-- Two schedules on the same route at the same time of day may never be in force together: the
-- exclusion constraint is that invariant, so a split or a concurrent create cannot leave the route
-- with two answers to "what time does it leave on the 5th". btree_gist is what lets the equality
-- halves (route_id, start_time) sit in a GIST index beside the date range; it is already installed
-- by 20260926000001, and asking again costs nothing.

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ===========================================================================
-- Calendars — the school year, the public holidays, the shutdown weeks
-- ===========================================================================

-- A named set of dates a schedule can point at. organization_id is the school whose calendar this
-- is, and NULL for one the operator keeps for the whole tenant.
CREATE TABLE tracking.calendars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    organization_id UUID REFERENCES tracking.organizations(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Filter + order in one index, so a page is not a sort of the whole table; the trigram index serves
-- the '%term%' search, which no b-tree can.
CREATE INDEX idx_calendars_tenant_name ON tracking.calendars (tenant_id, name, id);
CREATE INDEX idx_calendars_name_trgm ON tracking.calendars USING GIN (name gin_trgm_ops);
CREATE INDEX idx_calendars_organization_id ON tracking.calendars (organization_id)
    WHERE organization_id IS NOT NULL;

-- One date in one calendar. The primary key is the whole row's identity — a date appears once per
-- calendar or not at all — and it is the index the preview probes once per service date.
CREATE TABLE tracking.calendar_dates (
    calendar_id UUID NOT NULL REFERENCES tracking.calendars(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    kind VARCHAR(20) NOT NULL,
    label VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT pk_calendar_dates PRIMARY KEY (calendar_id, date),
    CONSTRAINT chk_calendar_date_kind CHECK (kind IN ('no_service', 'special_service'))
);

-- ===========================================================================
-- Schedules — the recurrence itself
-- ===========================================================================

CREATE TABLE tracking.route_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    -- Bit 0 Monday … bit 6 Sunday (D1). 0 would be a schedule that never runs, which is a delete.
    days_of_week SMALLINT NOT NULL,
    start_time TIME NOT NULL,
    valid_from DATE NOT NULL,
    valid_until DATE,
    calendar_id UUID REFERENCES tracking.calendars(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_route_schedule_days CHECK (days_of_week BETWEEN 1 AND 127),
    CONSTRAINT chk_route_schedule_range CHECK (valid_until IS NULL OR valid_until >= valid_from),
    CONSTRAINT ex_route_schedule_overlap EXCLUDE USING GIST (
        route_id WITH =,
        start_time WITH =,
        daterange(valid_from, valid_until, '[]') WITH &&
    )
);

-- The exclusion constraint's GIST index answers the preview's range join; this b-tree answers the
-- plain "this route's schedules, oldest first" the editor asks for.
CREATE INDEX idx_route_schedules_route_from ON tracking.route_schedules (route_id, valid_from);
CREATE INDEX idx_route_schedules_calendar ON tracking.route_schedules (calendar_id)
    WHERE calendar_id IS NOT NULL;

-- ===========================================================================
-- Exceptions — one day, or a few, that do not follow the recurrence
-- ===========================================================================

-- The payload's shape depends on the kind and is checked by the SP that writes it, not here: a
-- CHECK over JSONB would have to be repeated in seven branches and could not name the key it
-- refused. date_from = date_to is the common case — a single day off.
CREATE TABLE tracking.route_exceptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    date_from DATE NOT NULL,
    date_to DATE NOT NULL,
    kind VARCHAR(20) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    reason VARCHAR(500),
    created_by UUID,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_route_exception_range CHECK (date_from <= date_to),
    CONSTRAINT chk_route_exception_kind CHECK (kind IN (
        'cancel', 'change_time', 'change_vehicle', 'change_driver',
        'skip_stop', 'add_stop', 'detour'
    ))
);

-- The preview asks "which exceptions cover this date" once per date, so the range is what the index
-- holds; the uuid equality half again needs btree_gist.
CREATE INDEX idx_route_exceptions_route_range ON tracking.route_exceptions
    USING GIST (route_id, daterange(date_from, date_to, '[]'));

COMMENT ON TABLE tracking.calendars IS
'A named set of dates — a school year, the public holidays — a schedule points at so it produces no service on them (TRACK-018)';

COMMENT ON TABLE tracking.calendar_dates IS
'One date of one calendar: no_service suppresses the run, special_service adds one the weekday mask would not have produced';

COMMENT ON TABLE tracking.route_schedules IS
'When a route runs: a weekday bitmask (bit 0 Monday) and a start time, over a range of dates (TRACK-018 D1); the exclusion constraint forbids two schedules of the same route at the same time of day being in force together';

COMMENT ON TABLE tracking.route_exceptions IS
'What happens differently on one day or a few: the kind says what changes and the payload carries it, checked per kind by the SP that writes it (TRACK-018)';

COMMENT ON COLUMN tracking.route_schedules.days_of_week IS
'Weekday bitmask, bit 0 = Monday … bit 6 = Sunday (TRACK-018 D1); the same integer travels in JSON';
