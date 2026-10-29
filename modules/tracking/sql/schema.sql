--
-- PostgreSQL database dump
--


SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tracking; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA tracking;


--
-- Name: route_alerts; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.route_alerts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    route_id uuid,
    vehicle_id uuid,
    alert_type character varying(50) NOT NULL,
    severity character varying(50),
    message text,
    status character varying(50) DEFAULT 'active'::character varying,
    resolved_at timestamp without time zone,
    created_by uuid,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    title character varying(255),
    estimated_delay_minutes integer,
    trip_id uuid,
    location public.geography(Point,4326),
    resolved_by uuid,
    tenant_id uuid NOT NULL,
    CONSTRAINT chk_alert_type CHECK (((alert_type)::text = ANY ((ARRAY['delay'::character varying, 'breakdown'::character varying, 'cancellation'::character varying, 'emergency'::character varying, 'other'::character varying, 'device_silent'::character varying, 'off_route'::character varying])::text[])))
);


--
-- Name: COLUMN route_alerts.route_id; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.route_alerts.route_id IS 'NULL: an alert to the whole operation (TRACK-071)';


--
-- Name: drivers; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.drivers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    user_id uuid,
    first_name character varying(255) NOT NULL,
    last_name character varying(255) NOT NULL,
    phone character varying(50),
    license_number character varying(100),
    license_class character varying(50),
    license_expires_on date,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_driver_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying, 'suspended'::character varying])::text[])))
);


--
-- Name: TABLE drivers; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.drivers IS 'The people who drive a carrier''s vehicles, scoped to one tenant; user_id links the driver to a tenant account whose access level is `driver` (TRACK-006 D1), and is free of an FK because auth.users may live in another database';


--
-- Name: trips; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.trips (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    route_id uuid NOT NULL,
    route_version_id uuid,
    route_schedule_id uuid,
    service_date date NOT NULL,
    timezone character varying(64) NOT NULL,
    planned_start timestamp with time zone NOT NULL,
    vehicle_id uuid,
    driver_id uuid,
    status character varying(20) DEFAULT 'planned'::character varying NOT NULL,
    started_at timestamp with time zone,
    ended_at timestamp with time zone,
    is_overridden boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    polyline text,
    distance_km numeric(8,2),
    trace_source character varying(20),
    off_route_fixes smallint DEFAULT 0 NOT NULL,
    CONSTRAINT chk_trips_status CHECK (((status)::text = ANY ((ARRAY['planned'::character varying, 'in_progress'::character varying, 'completed'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: TABLE trips; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.trips IS 'One departure of a route on one service day, keyed by schedule and local date (TRACK-008); built by sp_materialise_trips from the TRACK-018 plan';


--
-- Name: COLUMN trips.polyline; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.trips.polyline IS 'The driven path at trip close, polyline precision 6 (TRACK-010)';


--
-- Name: COLUMN trips.trace_source; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.trips.trace_source IS 'valhalla (map-matched) or fallback (the raw GPS line)';


--
-- Name: COLUMN trips.off_route_fixes; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.trips.off_route_fixes IS 'Consecutive fixes off the planned line (positive) or back on it (negative): the hysteresis of the off_route alert (TRACK-071)';


--
-- Name: alert_settings; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.alert_settings (
    tenant_id uuid NOT NULL,
    device_silent_min smallint DEFAULT 5 NOT NULL,
    off_route_m smallint DEFAULT 150 NOT NULL,
    off_route_fixes smallint DEFAULT 3 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_alert_settings_off_route_fixes CHECK (((off_route_fixes >= 2) AND (off_route_fixes <= 10))),
    CONSTRAINT chk_alert_settings_off_route_m CHECK (((off_route_m >= 50) AND (off_route_m <= 1000))),
    CONSTRAINT chk_alert_settings_silent CHECK (((device_silent_min >= 1) AND (device_silent_min <= 60)))
);


--
-- Name: TABLE alert_settings; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.alert_settings IS 'The thresholds of the two alerts the server raises (TRACK-071 D1): no position for device_silent_min during a trip; off_route_fixes fixes in a row farther than off_route_m from the planned line. No row: 5 / 150 / 3';


--
-- Name: calendar_dates; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.calendar_dates (
    calendar_id uuid NOT NULL,
    date date NOT NULL,
    kind character varying(20) NOT NULL,
    label character varying(255),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_calendar_date_kind CHECK (((kind)::text = ANY ((ARRAY['no_service'::character varying, 'special_service'::character varying])::text[])))
);


--
-- Name: TABLE calendar_dates; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.calendar_dates IS 'One date of one calendar: no_service suppresses the run, special_service adds one the weekday mask would not have produced';


--
-- Name: calendars; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.calendars (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    organization_id uuid,
    name character varying(255) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    scope character varying(12) DEFAULT 'schedule'::character varying NOT NULL,
    CONSTRAINT chk_calendar_scope CHECK (((scope)::text = ANY ((ARRAY['schedule'::character varying, 'tenant'::character varying, 'organization'::character varying])::text[])))
);


--
-- Name: TABLE calendars; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.calendars IS 'A named set of dates — a school year, the public holidays — a schedule points at so it produces no service on them (TRACK-018)';


--
-- Name: COLUMN calendars.scope; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.calendars.scope IS 'schedule: read by the schedules that point at it · tenant: read by every route · organization: read by every route of organization_id (TRACK-076 D1)';


--
-- Name: compliance_documents; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.compliance_documents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    subject_type character varying(20) NOT NULL,
    subject_id uuid NOT NULL,
    document_type_id uuid NOT NULL,
    number character varying(100),
    issued_on date,
    expires_on date,
    file_name character varying(255),
    file_size bigint,
    file_ext character varying(10),
    notes text,
    warned_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_document_subject_type CHECK (((subject_type)::text = ANY ((ARRAY['vehicle'::character varying, 'driver'::character varying])::text[])))
);


--
-- Name: TABLE compliance_documents; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.compliance_documents IS 'One document against one vehicle or driver; subject_type says which table subject_id points at, and warned_at is the expiry digest''s "already said so" mark (TRACK-016 D1)';


--
-- Name: document_types; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.document_types (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(255) NOT NULL,
    applies_to character varying(20) DEFAULT 'both'::character varying NOT NULL,
    warn_days_before integer DEFAULT 30 NOT NULL,
    blocks_service boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_document_type_applies_to CHECK (((applies_to)::text = ANY ((ARRAY['vehicle'::character varying, 'driver'::character varying, 'both'::character varying])::text[]))),
    CONSTRAINT chk_document_type_warn_days CHECK (((warn_days_before >= 0) AND (warn_days_before <= 365)))
);


--
-- Name: TABLE document_types; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.document_types IS 'A tenant''s compliance policy: what documents a vehicle or driver must carry, how early to warn before one expires, and whether an expired one stops them working (TRACK-016)';


--
-- Name: driver_ops; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.driver_ops (
    client_op_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    driver_id uuid NOT NULL,
    trip_id uuid,
    op character varying(30) NOT NULL,
    args jsonb DEFAULT '{}'::jsonb NOT NULL,
    client_recorded_at timestamp with time zone NOT NULL,
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    status character varying(20) NOT NULL,
    code character varying(100),
    CONSTRAINT chk_driver_ops_status CHECK (((status)::text = ANY ((ARRAY['applied'::character varying, 'rejected'::character varying])::text[])))
);


--
-- Name: TABLE driver_ops; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.driver_ops IS 'One row per op a driver''s phone synced, keyed by the phone''s client_op_id: the answer it got (applied, or rejected + code), which a replay returns unchanged (TRACK-011 D1)';


--
-- Name: notification_settings; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.notification_settings (
    user_id uuid NOT NULL,
    rider_id uuid NOT NULL,
    type character varying(20) NOT NULL,
    CONSTRAINT chk_notification_settings_type CHECK (((type)::text = ANY ((ARRAY['trip_started'::character varying, 'approaching'::character varying, 'boarded'::character varying, 'dropped_off'::character varying, 'no_show'::character varying, 'cancelled'::character varying, 'changed'::character varying, 'delay'::character varying, 'trip_progress'::character varying, 'rider_added'::character varying])::text[])))
);


--
-- Name: TABLE notification_settings; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.notification_settings IS 'The (user, rider, type) pushes a recipient muted; no row is on. A muted notice keeps its feed row (TRACK-012)';


--
-- Name: notifications; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.notifications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    rider_id uuid NOT NULL,
    type character varying(20) NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    dedupe_key text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    read_at timestamp with time zone,
    CONSTRAINT chk_notifications_type CHECK (((type)::text = ANY ((ARRAY['trip_started'::character varying, 'approaching'::character varying, 'boarded'::character varying, 'dropped_off'::character varying, 'no_show'::character varying, 'cancelled'::character varying, 'changed'::character varying, 'delay'::character varying, 'rider_added'::character varying])::text[])))
);


--
-- Name: TABLE notifications; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.notifications IS 'The in-app feed: one row per notice per recipient and rider, written by sp_notify_event; the push is a copy of a row (TRACK-012)';


--
-- Name: optimization_runs; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.optimization_runs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    input_hash text NOT NULL,
    params jsonb NOT NULL,
    input jsonb NOT NULL,
    status character varying(16) DEFAULT 'queued'::character varying NOT NULL,
    error_code character varying(100),
    result jsonb,
    applied_at timestamp with time zone,
    applied_by uuid,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_optimization_run_status CHECK (((status)::text = ANY ((ARRAY['queued'::character varying, 'running'::character varying, 'done'::character varying, 'failed'::character varying])::text[])))
);


--
-- Name: TABLE optimization_runs; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.optimization_runs IS 'One route proposal for a destination (TRACK-014): the problem asked (input, input_hash), its state queued|running|done|failed, VROOM''s answer as routes and unassigned riders (result), and when it was applied';


--
-- Name: organization_members; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.organization_members (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role character varying(20) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    groups text[],
    CONSTRAINT chk_organization_member_groups CHECK (((groups IS NULL) OR ((role)::text = 'supervisor'::text))),
    CONSTRAINT chk_organization_member_role CHECK (((role)::text = ANY ((ARRAY['admin'::character varying, 'viewer'::character varying, 'supervisor'::character varying])::text[])))
);


--
-- Name: TABLE organization_members; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.organization_members IS 'Which tenant users see an organization and with what role (TRACK-005 D3); the channel they may sign in through is TRACK-015''s access level, not a column here';


--
-- Name: COLUMN organization_members.groups; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.organization_members.groups IS 'The rider groups (riders.group_label) a supervisor is limited to; NULL = the whole organization, only a supervisor carries them (TRACK-067 D2)';


--
-- Name: organizations; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.organizations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    kind character varying(20) NOT NULL,
    name character varying(255) NOT NULL,
    timezone character varying(64) DEFAULT 'UTC'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    absence_cutoff_min integer DEFAULT 60 NOT NULL,
    location public.geography(Point,4326),
    address character varying(500),
    stop_place_id uuid,
    empty_trip_cancel_min integer DEFAULT 30 NOT NULL,
    delay_notice_min integer DEFAULT 10 NOT NULL,
    CONSTRAINT chk_organization_absence_cutoff CHECK (((absence_cutoff_min >= 0) AND (absence_cutoff_min <= 1440))),
    CONSTRAINT chk_organization_delay_notice CHECK (((delay_notice_min >= 0) AND (delay_notice_min <= 180))),
    CONSTRAINT chk_organization_empty_trip_cancel CHECK (((empty_trip_cancel_min >= 0) AND (empty_trip_cancel_min <= 1440))),
    CONSTRAINT chk_organization_kind CHECK (((kind)::text = ANY ((ARRAY['school'::character varying, 'company'::character varying, 'other'::character varying])::text[])))
);


--
-- Name: TABLE organizations; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.organizations IS 'Schools and employers whose people ride, scoped to one tenant; kind tells them apart and timezone is the IANA zone their schedules are read in';


--
-- Name: COLUMN organizations.location; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.organizations.location IS 'Where the school or company is: the destination of its routes (TRACK-038 D2)';


--
-- Name: COLUMN organizations.stop_place_id; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.organizations.stop_place_id IS 'The client''s own stop at its pin: the destination of its route pairs, kept in sync with the pin (TRACK-045 D3)';


--
-- Name: COLUMN organizations.empty_trip_cancel_min; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.organizations.empty_trip_cancel_min IS 'Minutes before planned_start a trip of this client with no rider cancels itself; 0 never (TRACK-069 D1)';


--
-- Name: COLUMN organizations.delay_notice_min; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.organizations.delay_notice_min IS 'Minutes of ETA past planned_at at a stop that send its pending riders'' families a delay notice (TRACK-066 D4)';


--
-- Name: outbox; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.outbox (
    id bigint NOT NULL,
    topic text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    tenant_id uuid NOT NULL
);


--
-- Name: TABLE outbox; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.outbox IS 'Side effects written in the transaction that causes them; the worker''s relay publishes each row as an asynq task outbox:<topic> and stamps published_at (INFRA-001 D3)';


--
-- Name: COLUMN outbox.tenant_id; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.outbox.tenant_id IS 'Whose event this is; the relay publishes the task as <tenant_id>:<id>, so tenants sharing a database never mix (INFRA-009)';


--
-- Name: outbox_id_seq; Type: SEQUENCE; Schema: tracking; Owner: -
--

CREATE SEQUENCE tracking.outbox_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: outbox_id_seq; Type: SEQUENCE OWNED BY; Schema: tracking; Owner: -
--

ALTER SEQUENCE tracking.outbox_id_seq OWNED BY tracking.outbox.id;


--
-- Name: rider_absences; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.rider_absences (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    rider_id uuid NOT NULL,
    date_from date NOT NULL,
    date_to date NOT NULL,
    direction character varying(20),
    reason character varying(500),
    reported_by uuid,
    reported_via character varying(10) DEFAULT 'web'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_rider_absence_direction CHECK (((direction IS NULL) OR ((direction)::text = ANY ((ARRAY['outbound'::character varying, 'inbound'::character varying])::text[])))),
    CONSTRAINT chk_rider_absence_range CHECK ((date_to >= date_from)),
    CONSTRAINT chk_rider_absence_via CHECK (((reported_via)::text = ANY ((ARRAY['web'::character varying, 'portal'::character varying])::text[])))
);


--
-- Name: rider_access_log; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.rider_access_log (
    id bigint NOT NULL,
    tenant_id uuid NOT NULL,
    rider_id uuid NOT NULL,
    user_id uuid NOT NULL,
    level character varying(20) NOT NULL,
    action character varying(20) NOT NULL,
    bucket timestamp with time zone NOT NULL,
    reads integer DEFAULT 1 NOT NULL,
    first_at timestamp with time zone NOT NULL,
    last_at timestamp with time zone NOT NULL,
    CONSTRAINT chk_rider_access_log_action CHECK (((action)::text = ANY ((ARRAY['status'::character varying, 'today'::character varying, 'live'::character varying, 'absences'::character varying])::text[]))),
    CONSTRAINT chk_rider_access_log_level CHECK (((level)::text = ANY ((ARRAY['portal'::character varying, 'organization'::character varying])::text[])))
);


--
-- Name: TABLE rider_access_log; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.rider_access_log IS 'Reads of a rider by a guardian (portal) or a client''s user (organization), one row per user, rider, action and hour; reads counts the calls in that hour (TRACK-063 D1, D2)';


--
-- Name: rider_access_log_id_seq; Type: SEQUENCE; Schema: tracking; Owner: -
--

CREATE SEQUENCE tracking.rider_access_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rider_access_log_id_seq; Type: SEQUENCE OWNED BY; Schema: tracking; Owner: -
--

ALTER SEQUENCE tracking.rider_access_log_id_seq OWNED BY tracking.rider_access_log.id;


--
-- Name: rider_contacts; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.rider_contacts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    rider_id uuid NOT NULL,
    relation character varying(20) NOT NULL,
    name character varying(255),
    phone character varying(50),
    email character varying(255),
    user_id uuid,
    is_primary boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_rider_contact_relation CHECK (((relation)::text = ANY ((ARRAY['guardian'::character varying, 'emergency'::character varying, 'self'::character varying])::text[])))
);


--
-- Name: rider_day_attendance; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.rider_day_attendance (
    trip_id uuid NOT NULL,
    rider_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    service_date date NOT NULL,
    direction character varying(20) NOT NULL,
    route_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    status character varying(20) NOT NULL,
    CONSTRAINT chk_rider_day_attendance_status CHECK (((status)::text = ANY ((ARRAY['boarded'::character varying, 'no_show'::character varying, 'absent'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: TABLE rider_day_attendance; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.rider_day_attendance IS 'Each rider''s pickup on an ended trip: boarded, no_show, absent (an absence covered the day) or cancelled (TRACK-013 D1)';


--
-- Name: rider_places; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.rider_places (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    rider_id uuid NOT NULL,
    label character varying(60) NOT NULL,
    address character varying(500),
    location public.geography(Point,4326) NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE rider_places; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.rider_places IS 'A rider''s saved places ("Casa", "Oficina"); the default one is mirrored into riders.home_location and address, which every other read uses (TRACK-044 D4)';


--
-- Name: rider_route_assignments; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.rider_route_assignments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    rider_id uuid NOT NULL,
    route_id uuid NOT NULL,
    days_of_week smallint NOT NULL,
    pickup_stop_place_id uuid,
    dropoff_stop_place_id uuid,
    valid_from date NOT NULL,
    valid_until date,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_rider_route_assignment_days CHECK (((days_of_week >= 1) AND (days_of_week <= 127))),
    CONSTRAINT chk_rider_route_assignment_range CHECK (((valid_until IS NULL) OR (valid_until >= valid_from)))
);


--
-- Name: TABLE rider_route_assignments; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.rider_route_assignments IS 'Which route a rider rides, on which weekdays, between which dates; the direction is the route''s (TRACK-009 D2)';


--
-- Name: COLUMN rider_route_assignments.days_of_week; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.rider_route_assignments.days_of_week IS 'Bitmask, bit 0 Monday … bit 6 Sunday; 1..127';


--
-- Name: COLUMN rider_route_assignments.valid_until; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.rider_route_assignments.valid_until IS 'Last day the assignment is in force, inclusive; NULL is open-ended';


--
-- Name: riders; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.riders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    first_name character varying(100) NOT NULL,
    last_name character varying(100) NOT NULL,
    email character varying(255),
    phone character varying(20),
    identification_number character varying(50),
    status character varying(50) DEFAULT 'active'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    rider_type character varying(50) DEFAULT 'employee'::character varying NOT NULL,
    organization_id uuid NOT NULL,
    home_location public.geography(Point,4326),
    notes text,
    qr_token text DEFAULT tracking.fn_new_qr_token() NOT NULL,
    group_label character varying(100),
    address character varying(500),
    service_legs character varying(10) DEFAULT 'both'::character varying NOT NULL,
    stop_place_id uuid,
    home_geocoded boolean DEFAULT false NOT NULL,
    deactivated_at timestamp with time zone,
    anonymised_at timestamp with time zone,
    CONSTRAINT check_rider_type CHECK (((rider_type)::text = ANY ((ARRAY['student'::character varying, 'employee'::character varying])::text[]))),
    CONSTRAINT chk_rider_service_legs CHECK (((service_legs)::text = ANY ((ARRAY['both'::character varying, 'outbound'::character varying, 'return'::character varying])::text[])))
);


--
-- Name: COLUMN riders.home_geocoded; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.riders.home_geocoded IS 'The home pin came from geocoding the address at import, not from a person; moving the pin clears it (TRACK-072 D3)';


--
-- Name: COLUMN riders.deactivated_at; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.riders.deactivated_at IS 'When the rider last turned inactive, NULL while active; the contact retention counts from here (TRACK-063 D3)';


--
-- Name: COLUMN riders.anonymised_at; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.riders.anonymised_at IS 'When the retention pass cleared the rider''s contact data (TRACK-063 D3)';


--
-- Name: route_exceptions; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.route_exceptions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    route_id uuid NOT NULL,
    date_from date NOT NULL,
    date_to date NOT NULL,
    kind character varying(20) NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    reason character varying(500),
    created_by uuid,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_route_exception_kind CHECK (((kind)::text = ANY ((ARRAY['cancel'::character varying, 'change_time'::character varying, 'change_vehicle'::character varying, 'change_driver'::character varying, 'skip_stop'::character varying, 'add_stop'::character varying, 'detour'::character varying])::text[]))),
    CONSTRAINT chk_route_exception_range CHECK ((date_from <= date_to))
);


--
-- Name: TABLE route_exceptions; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.route_exceptions IS 'What happens differently on one day or a few: the kind says what changes and the payload carries it, checked per kind by the SP that writes it (TRACK-018)';


--
-- Name: route_schedules; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.route_schedules (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    route_id uuid NOT NULL,
    days_of_week smallint NOT NULL,
    start_time time without time zone NOT NULL,
    valid_from date NOT NULL,
    valid_until date,
    calendar_id uuid,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    departure_auto_min integer,
    CONSTRAINT chk_route_schedule_days CHECK (((days_of_week >= 1) AND (days_of_week <= 127))),
    CONSTRAINT chk_route_schedule_range CHECK (((valid_until IS NULL) OR (valid_until >= valid_from)))
);


--
-- Name: TABLE route_schedules; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.route_schedules IS 'When a route runs: a weekday bitmask (bit 0 Monday) and a start time, over a range of dates (TRACK-018 D1); the exclusion constraint forbids two schedules of the same route at the same time of day being in force together';


--
-- Name: COLUMN route_schedules.days_of_week; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.route_schedules.days_of_week IS 'Weekday bitmask, bit 0 = Monday … bit 6 = Sunday (TRACK-018 D1); the same integer travels in JSON';


--
-- Name: COLUMN route_schedules.departure_auto_min; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.route_schedules.departure_auto_min IS 'Minutes this departure leaves before the arrival the route pair was created for, while the estimate owns it; NULL once a user sets the time (TRACK-044 D2)';


--
-- Name: route_version_stops; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.route_version_stops (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    version_id uuid NOT NULL,
    stop_place_id uuid NOT NULL,
    sequence integer NOT NULL,
    planned_offset_min integer,
    dwell_sec integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_route_version_stop_sequence CHECK ((sequence >= 1))
);


--
-- Name: TABLE route_version_stops; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.route_version_stops IS 'The ordered stops of one route version; a reorder replaces the whole list in one request rather than renumbering rows';


--
-- Name: route_versions; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.route_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    route_id uuid NOT NULL,
    effective_from date NOT NULL,
    effective_to date,
    created_by uuid,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    planned_polyline text,
    planned_distance_m integer,
    planned_legs jsonb,
    path_source character varying(20),
    path_points_hash character varying(64),
    path_computed_at timestamp with time zone,
    planned_line public.geography(LineString,4326),
    CONSTRAINT chk_route_version_range CHECK (((effective_to IS NULL) OR (effective_to >= effective_from))),
    CONSTRAINT chk_route_versions_path_source CHECK (((path_source)::text = ANY ((ARRAY['valhalla'::character varying, 'fallback'::character varying])::text[])))
);


--
-- Name: TABLE route_versions; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.route_versions IS 'One route''s stop list over a range of dates; the exclusion constraint guarantees at most one version is in force per route per day (TRACK-007)';


--
-- Name: COLUMN route_versions.planned_polyline; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.route_versions.planned_polyline IS 'The line through the version''s stops by streets, polyline precision 6 (TRACK-028)';


--
-- Name: COLUMN route_versions.planned_legs; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.route_versions.planned_legs IS '[{distance_m, duration_s}] per stop-to-stop leg, in sequence';


--
-- Name: COLUMN route_versions.path_source; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.route_versions.path_source IS 'valhalla (by streets) or fallback (straight segments, retried)';


--
-- Name: COLUMN route_versions.path_points_hash; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.route_versions.path_points_hash IS 'Hash of the ordered coordinates the line was computed from: equal means nothing to recompute';


--
-- Name: COLUMN route_versions.planned_line; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.route_versions.planned_line IS 'planned_polyline as geography, for the off-route check; NULL for a fallback (straight) line, which has no street to be off (TRACK-071)';


--
-- Name: routes; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.routes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    company_id uuid NOT NULL,
    status character varying(50) DEFAULT 'active'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    vehicle_id uuid,
    route_name character varying(255) NOT NULL,
    route_code character varying(50),
    origin_address character varying(500) NOT NULL,
    origin_lat numeric(10,8),
    origin_lng numeric(11,8),
    destination_address character varying(500) NOT NULL,
    destination_lat numeric(10,8),
    destination_lng numeric(11,8),
    schedule_type character varying(50) DEFAULT 'custom'::character varying,
    scheduled_start_time time without time zone,
    scheduled_end_time time without time zone,
    estimated_duration_minutes integer,
    is_active boolean DEFAULT true,
    default_driver_id uuid,
    direction character varying(20) DEFAULT 'outbound'::character varying NOT NULL,
    timezone character varying(64) DEFAULT 'UTC'::character varying NOT NULL,
    capacity integer,
    organization_id uuid,
    paired_route_id uuid,
    CONSTRAINT chk_route_capacity CHECK (((capacity IS NULL) OR ((capacity >= 1) AND (capacity <= 200)))),
    CONSTRAINT chk_routes_direction CHECK (((direction)::text = ANY ((ARRAY['outbound'::character varying, 'inbound'::character varying])::text[])))
);


--
-- Name: COLUMN routes.default_driver_id; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.routes.default_driver_id IS 'The driver who normally drives this route; NULL when none is set, and set to NULL rather than cascading when that driver is deleted';


--
-- Name: COLUMN routes.direction; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.routes.direction IS 'Which way this route runs: outbound leaves the depot, inbound returns (TRACK-007)';


--
-- Name: COLUMN routes.timezone; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.routes.timezone IS 'IANA zone the route runs in: its trips'' service_date is the local day and planned_start is service_date + start time in this zone (TRACK-008 D2)';


--
-- Name: COLUMN routes.capacity; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.routes.capacity IS 'Seats this route offers when it differs from its vehicle''s; NULL is the vehicle''s (TRACK-023 D2)';


--
-- Name: COLUMN routes.paired_route_id; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.routes.paired_route_id IS 'The other leg of an outbound/return pair (TRACK-038 D1); each route points at the other';


--
-- Name: share_links; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.share_links (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    trip_id uuid NOT NULL,
    rider_id uuid,
    token_hash bytea NOT NULL,
    created_by uuid,
    created_via character varying(10) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_share_links_via CHECK (((created_via)::text = ANY ((ARRAY['web'::character varying, 'portal'::character varying])::text[])))
);


--
-- Name: TABLE share_links; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.share_links IS 'Public links to a trip: the sha256 of each token, who made it and until when it opens (TRACK-060)';


--
-- Name: stop_delay_stats; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.stop_delay_stats (
    route_id uuid NOT NULL,
    stop_place_id uuid NOT NULL,
    weekday smallint NOT NULL,
    tenant_id uuid NOT NULL,
    samples integer NOT NULL,
    p50_delay_min numeric(6,1) NOT NULL,
    p90_delay_min numeric(6,1) NOT NULL,
    computed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_stop_delay_stats_weekday CHECK (((weekday >= 1) AND (weekday <= 7)))
);


--
-- Name: TABLE stop_delay_stats; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.stop_delay_stats IS 'Minutes each stop of a route is reached past planned_at, p50 and p90 by ISO weekday, over completed trips of the last weeks (TRACK-013 D5)';


--
-- Name: stop_pair_samples; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.stop_pair_samples (
    trip_id uuid NOT NULL,
    from_stop_place_id uuid NOT NULL,
    to_stop_place_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    weekday smallint NOT NULL,
    hour_band smallint NOT NULL,
    duration_s integer NOT NULL,
    observed_at timestamp with time zone NOT NULL,
    CONSTRAINT chk_stop_pair_samples_duration CHECK ((duration_s > 0)),
    CONSTRAINT chk_stop_pair_samples_hour_band CHECK (((hour_band >= 0) AND (hour_band <= 11))),
    CONSTRAINT chk_stop_pair_samples_weekday CHECK (((weekday >= 1) AND (weekday <= 7)))
);


--
-- Name: TABLE stop_pair_samples; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.stop_pair_samples IS 'Observed arrival-to-arrival seconds between consecutive stops of completed trips: the ETA source when the router is down (TRACK-066 D1)';


--
-- Name: stop_places; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.stop_places (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    organization_id uuid,
    name character varying(255) NOT NULL,
    address character varying(500),
    location public.geography(Point,4326),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE stop_places; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.stop_places IS 'A place a route calls at, named once and shared by every route that stops there (TRACK-007 D1); location is a PostGIS point so "near here" is an index scan';


--
-- Name: transport_companies; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.transport_companies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    email character varying(255),
    phone character varying(20),
    address text,
    city character varying(100),
    country character varying(100),
    registration_number character varying(100) NOT NULL,
    status character varying(50) DEFAULT 'active'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: trip_events; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.trip_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    trip_id uuid NOT NULL,
    type character varying(50) NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid
);


--
-- Name: TABLE trip_events; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.trip_events IS 'A trip''s audit trail; ride_events rows migrated by TRACK-008 carry the whole legacy row under payload.ride_event';


--
-- Name: trip_stop_tasks; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.trip_stop_tasks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    trip_stop_id uuid NOT NULL,
    kind character varying(20) NOT NULL,
    subject_type character varying(20) DEFAULT 'passenger'::character varying NOT NULL,
    subject_id uuid NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    done_at timestamp with time zone,
    done_by uuid,
    client_op_id uuid,
    proof jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_trip_stop_tasks_kind CHECK (((kind)::text = ANY ((ARRAY['pickup'::character varying, 'dropoff'::character varying])::text[]))),
    CONSTRAINT chk_trip_stop_tasks_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'done'::character varying, 'no_show'::character varying, 'cancelled'::character varying])::text[]))),
    CONSTRAINT chk_trip_stop_tasks_subject CHECK (((subject_type)::text = ANY ((ARRAY['passenger'::character varying, 'parcel'::character varying])::text[])))
);


--
-- Name: TABLE trip_stop_tasks; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.trip_stop_tasks IS 'What happens at a stop: a pickup or dropoff of one subject (ARCH-001 D1: parcel is reserved, refused by every SP until a delivery module exists)';


--
-- Name: trip_stops; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.trip_stops (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    trip_id uuid NOT NULL,
    stop_place_id uuid NOT NULL,
    sequence integer NOT NULL,
    planned_at timestamp with time zone,
    arrived_at timestamp with time zone,
    departed_at timestamp with time zone,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    approach_notified_at timestamp with time zone,
    delay_notified_at timestamp with time zone,
    CONSTRAINT chk_trip_stops_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'arrived'::character varying, 'skipped'::character varying])::text[])))
);


--
-- Name: TABLE trip_stops; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.trip_stops IS 'The stops of one trip, copied from the route version in force on its service date';


--
-- Name: COLUMN trip_stops.approach_notified_at; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.trip_stops.approach_notified_at IS 'When the stop''s approach notice went out: the first position within the approach radius while it was the trip''s next pending stop (TRACK-012 D4)';


--
-- Name: COLUMN trip_stops.delay_notified_at; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.trip_stops.delay_notified_at IS 'When the stop''s late-ETA notice went out: the first fresh ETA past the threshold of its riders'' clients (TRACK-066 D3)';


--
-- Name: trip_summaries; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.trip_summaries (
    trip_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    service_date date NOT NULL,
    route_id uuid NOT NULL,
    organization_id uuid,
    organization_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    vehicle_id uuid,
    driver_id uuid,
    status character varying(20) NOT NULL,
    planned_start timestamp with time zone NOT NULL,
    started_at timestamp with time zone,
    ended_at timestamp with time zone,
    distance_km numeric(8,2),
    duration_min integer,
    tasks_planned integer DEFAULT 0 NOT NULL,
    pickups_done integer DEFAULT 0 NOT NULL,
    no_shows integer DEFAULT 0 NOT NULL,
    cancelled integer DEFAULT 0 NOT NULL,
    max_onboard integer DEFAULT 0 NOT NULL,
    capacity integer,
    start_delay_min integer,
    avg_stop_delay_min numeric(6,1),
    max_stop_delay_min integer,
    stops_late integer DEFAULT 0 NOT NULL,
    alerts_count integer DEFAULT 0 NOT NULL,
    rolled_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_trip_summaries_status CHECK (((status)::text = ANY ((ARRAY['completed'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: TABLE trip_summaries; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.trip_summaries IS 'One row per ended trip: km, minutes, riders, lateness and alerts, rolled at trip close and by the nightly sweep (TRACK-013 D1)';


--
-- Name: COLUMN trip_summaries.organization_ids; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.trip_summaries.organization_ids IS 'The organizations of the trip''s riders: what an organization user''s scope matches (TRACK-013 D4)';


--
-- Name: COLUMN trip_summaries.max_onboard; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.trip_summaries.max_onboard IS 'The most riders aboard at once: done pickups minus done dropoffs, in done_at order';


--
-- Name: COLUMN trip_summaries.stops_late; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.trip_summaries.stops_late IS 'Stops arrived more than 5 minutes after planned_at';


--
-- Name: vehicle_last_position; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.vehicle_last_position (
    vehicle_id uuid NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    location public.geography(Point,4326) NOT NULL,
    speed real,
    heading real,
    accuracy real,
    trip_id uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    source character varying(10) DEFAULT 'phone'::character varying NOT NULL,
    CONSTRAINT chk_vehicle_last_position_source CHECK (((source)::text = ANY ((ARRAY['phone'::character varying, 'device'::character varying])::text[])))
);


--
-- Name: TABLE vehicle_last_position; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.vehicle_last_position IS 'The newest point of each vehicle — what every live read looks up (TRACK-010)';


--
-- Name: COLUMN vehicle_last_position.source; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.vehicle_last_position.source IS 'phone: the driver app · device: a hardware tracker (OsmAnd protocol); a device point is dropped during a trip while the phone reported within GPS_PHONE_PRIORITY_S (TRACK-068 D2)';


--
-- Name: vehicle_positions; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.vehicle_positions (
    vehicle_id uuid NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    location public.geography(Point,4326) NOT NULL,
    speed real,
    heading real,
    accuracy real,
    trip_id uuid,
    source character varying(10) DEFAULT 'phone'::character varying NOT NULL,
    CONSTRAINT chk_vehicle_positions_source CHECK (((source)::text = ANY ((ARRAY['phone'::character varying, 'device'::character varying])::text[])))
)
PARTITION BY RANGE (recorded_at);


--
-- Name: TABLE vehicle_positions; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON TABLE tracking.vehicle_positions IS 'Every GPS point, one partition per UTC day (TRACK-010 D1); written only by sp_ingest_positions, dropped by sp_positions_partitions past retention';


--
-- Name: vehicle_positions_default; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.vehicle_positions_default (
    vehicle_id uuid NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    location public.geography(Point,4326) NOT NULL,
    speed real,
    heading real,
    accuracy real,
    trip_id uuid,
    source character varying(10) DEFAULT 'phone'::character varying NOT NULL,
    CONSTRAINT chk_vehicle_positions_source CHECK (((source)::text = ANY ((ARRAY['phone'::character varying, 'device'::character varying])::text[])))
);


--
-- Name: vehicles; Type: TABLE; Schema: tracking; Owner: -
--

CREATE TABLE tracking.vehicles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    company_id uuid NOT NULL,
    plate_number character varying(50) NOT NULL,
    vehicle_type character varying(50) NOT NULL,
    brand character varying(100),
    model character varying(100),
    year integer,
    capacity integer,
    gps_device_id character varying(100),
    status character varying(50) DEFAULT 'active'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    default_driver_id uuid,
    CONSTRAINT chk_vehicle_type CHECK (((vehicle_type)::text = ANY ((ARRAY['bus'::character varying, 'van'::character varying, 'car'::character varying])::text[])))
);


--
-- Name: COLUMN vehicles.default_driver_id; Type: COMMENT; Schema: tracking; Owner: -
--

COMMENT ON COLUMN tracking.vehicles.default_driver_id IS 'The driver who usually drives this vehicle; a route without its own driver runs with this one (TRACK-034 D1)';


--
-- Name: vehicle_positions_default; Type: TABLE ATTACH; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicle_positions ATTACH PARTITION tracking.vehicle_positions_default DEFAULT;


--
-- Name: outbox id; Type: DEFAULT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.outbox ALTER COLUMN id SET DEFAULT nextval('tracking.outbox_id_seq'::regclass);


--
-- Name: rider_access_log id; Type: DEFAULT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_access_log ALTER COLUMN id SET DEFAULT nextval('tracking.rider_access_log_id_seq'::regclass);


--
-- Name: alert_settings alert_settings_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.alert_settings
    ADD CONSTRAINT alert_settings_pkey PRIMARY KEY (tenant_id);


--
-- Name: calendars calendars_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.calendars
    ADD CONSTRAINT calendars_pkey PRIMARY KEY (id);


--
-- Name: compliance_documents compliance_documents_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.compliance_documents
    ADD CONSTRAINT compliance_documents_pkey PRIMARY KEY (id);


--
-- Name: document_types document_types_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.document_types
    ADD CONSTRAINT document_types_pkey PRIMARY KEY (id);


--
-- Name: driver_ops driver_ops_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.driver_ops
    ADD CONSTRAINT driver_ops_pkey PRIMARY KEY (client_op_id);


--
-- Name: drivers drivers_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.drivers
    ADD CONSTRAINT drivers_pkey PRIMARY KEY (id);


--
-- Name: route_schedules ex_route_schedule_overlap; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_schedules
    ADD CONSTRAINT ex_route_schedule_overlap EXCLUDE USING gist (route_id WITH =, start_time WITH =, daterange(valid_from, valid_until, '[]'::text) WITH &&);


--
-- Name: route_versions ex_route_version_overlap; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_versions
    ADD CONSTRAINT ex_route_version_overlap EXCLUDE USING gist (route_id WITH =, daterange(effective_from, effective_to, '[]'::text) WITH &&);


--
-- Name: notification_settings notification_settings_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.notification_settings
    ADD CONSTRAINT notification_settings_pkey PRIMARY KEY (user_id, rider_id, type);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: optimization_runs optimization_runs_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.optimization_runs
    ADD CONSTRAINT optimization_runs_pkey PRIMARY KEY (id);


--
-- Name: organization_members organization_members_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.organization_members
    ADD CONSTRAINT organization_members_pkey PRIMARY KEY (id);


--
-- Name: organizations organizations_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);


--
-- Name: outbox outbox_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.outbox
    ADD CONSTRAINT outbox_pkey PRIMARY KEY (id);


--
-- Name: calendar_dates pk_calendar_dates; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.calendar_dates
    ADD CONSTRAINT pk_calendar_dates PRIMARY KEY (calendar_id, date);


--
-- Name: rider_absences rider_absences_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_absences
    ADD CONSTRAINT rider_absences_pkey PRIMARY KEY (id);


--
-- Name: rider_access_log rider_access_log_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_access_log
    ADD CONSTRAINT rider_access_log_pkey PRIMARY KEY (id);


--
-- Name: rider_contacts rider_contacts_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_contacts
    ADD CONSTRAINT rider_contacts_pkey PRIMARY KEY (id);


--
-- Name: rider_day_attendance rider_day_attendance_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_day_attendance
    ADD CONSTRAINT rider_day_attendance_pkey PRIMARY KEY (trip_id, rider_id);


--
-- Name: rider_places rider_places_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_places
    ADD CONSTRAINT rider_places_pkey PRIMARY KEY (id);


--
-- Name: rider_route_assignments rider_route_assignments_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_route_assignments
    ADD CONSTRAINT rider_route_assignments_pkey PRIMARY KEY (id);


--
-- Name: riders riders_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.riders
    ADD CONSTRAINT riders_pkey PRIMARY KEY (id);


--
-- Name: route_alerts route_alerts_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_alerts
    ADD CONSTRAINT route_alerts_pkey PRIMARY KEY (id);


--
-- Name: route_exceptions route_exceptions_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_exceptions
    ADD CONSTRAINT route_exceptions_pkey PRIMARY KEY (id);


--
-- Name: route_schedules route_schedules_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_schedules
    ADD CONSTRAINT route_schedules_pkey PRIMARY KEY (id);


--
-- Name: route_version_stops route_version_stops_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_version_stops
    ADD CONSTRAINT route_version_stops_pkey PRIMARY KEY (id);


--
-- Name: route_versions route_versions_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_versions
    ADD CONSTRAINT route_versions_pkey PRIMARY KEY (id);


--
-- Name: routes routes_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.routes
    ADD CONSTRAINT routes_pkey PRIMARY KEY (id);


--
-- Name: share_links share_links_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.share_links
    ADD CONSTRAINT share_links_pkey PRIMARY KEY (id);


--
-- Name: share_links share_links_token_hash_key; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.share_links
    ADD CONSTRAINT share_links_token_hash_key UNIQUE (token_hash);


--
-- Name: stop_delay_stats stop_delay_stats_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_delay_stats
    ADD CONSTRAINT stop_delay_stats_pkey PRIMARY KEY (route_id, stop_place_id, weekday);


--
-- Name: stop_pair_samples stop_pair_samples_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_pair_samples
    ADD CONSTRAINT stop_pair_samples_pkey PRIMARY KEY (trip_id, from_stop_place_id);


--
-- Name: stop_places stop_places_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_places
    ADD CONSTRAINT stop_places_pkey PRIMARY KEY (id);


--
-- Name: transport_companies transport_companies_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.transport_companies
    ADD CONSTRAINT transport_companies_pkey PRIMARY KEY (id);


--
-- Name: trip_events trip_events_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_events
    ADD CONSTRAINT trip_events_pkey PRIMARY KEY (id);


--
-- Name: trip_stop_tasks trip_stop_tasks_client_op_id_key; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_stop_tasks
    ADD CONSTRAINT trip_stop_tasks_client_op_id_key UNIQUE (client_op_id);


--
-- Name: trip_stop_tasks trip_stop_tasks_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_stop_tasks
    ADD CONSTRAINT trip_stop_tasks_pkey PRIMARY KEY (id);


--
-- Name: trip_stops trip_stops_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_stops
    ADD CONSTRAINT trip_stops_pkey PRIMARY KEY (id);


--
-- Name: trip_summaries trip_summaries_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_summaries
    ADD CONSTRAINT trip_summaries_pkey PRIMARY KEY (trip_id);


--
-- Name: trips trips_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trips
    ADD CONSTRAINT trips_pkey PRIMARY KEY (id);


--
-- Name: document_types uk_document_type_code; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.document_types
    ADD CONSTRAINT uk_document_type_code UNIQUE (tenant_id, code);


--
-- Name: drivers uk_driver_license; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.drivers
    ADD CONSTRAINT uk_driver_license UNIQUE (tenant_id, license_number);


--
-- Name: drivers uk_driver_user; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.drivers
    ADD CONSTRAINT uk_driver_user UNIQUE (tenant_id, user_id);


--
-- Name: organization_members uk_organization_member; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.organization_members
    ADD CONSTRAINT uk_organization_member UNIQUE (organization_id, user_id);


--
-- Name: riders uk_riders_qr_token; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.riders
    ADD CONSTRAINT uk_riders_qr_token UNIQUE (qr_token);


--
-- Name: route_version_stops uk_route_version_stop_sequence; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_version_stops
    ADD CONSTRAINT uk_route_version_stop_sequence UNIQUE (version_id, sequence);


--
-- Name: trip_stop_tasks uk_trip_stop_tasks_subject; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_stop_tasks
    ADD CONSTRAINT uk_trip_stop_tasks_subject UNIQUE (trip_stop_id, kind, subject_type, subject_id);


--
-- Name: trip_stops uk_trip_stops_sequence; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_stops
    ADD CONSTRAINT uk_trip_stops_sequence UNIQUE (trip_id, sequence);


--
-- Name: trips uk_trips_schedule_date; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trips
    ADD CONSTRAINT uk_trips_schedule_date UNIQUE (route_schedule_id, service_date);


--
-- Name: vehicles uk_vehicle_plate; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicles
    ADD CONSTRAINT uk_vehicle_plate UNIQUE (company_id, plate_number);


--
-- Name: rider_access_log uq_rider_access_log_bucket; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_access_log
    ADD CONSTRAINT uq_rider_access_log_bucket UNIQUE (rider_id, user_id, action, bucket);


--
-- Name: notifications ux_notifications_dedupe; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.notifications
    ADD CONSTRAINT ux_notifications_dedupe UNIQUE (user_id, rider_id, type, dedupe_key);


--
-- Name: vehicle_last_position vehicle_last_position_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicle_last_position
    ADD CONSTRAINT vehicle_last_position_pkey PRIMARY KEY (vehicle_id);


--
-- Name: vehicle_positions vehicle_positions_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicle_positions
    ADD CONSTRAINT vehicle_positions_pkey PRIMARY KEY (vehicle_id, recorded_at);


--
-- Name: vehicle_positions_default vehicle_positions_default_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicle_positions_default
    ADD CONSTRAINT vehicle_positions_default_pkey PRIMARY KEY (vehicle_id, recorded_at);


--
-- Name: vehicles vehicles_pkey; Type: CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicles
    ADD CONSTRAINT vehicles_pkey PRIMARY KEY (id);


--
-- Name: idx_calendars_name_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_calendars_name_trgm ON tracking.calendars USING gin (name public.gin_trgm_ops);


--
-- Name: idx_calendars_organization_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_calendars_organization_id ON tracking.calendars USING btree (organization_id) WHERE (organization_id IS NOT NULL);


--
-- Name: idx_calendars_tenant_name; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_calendars_tenant_name ON tracking.calendars USING btree (tenant_id, name, id);


--
-- Name: idx_documents_expires_on; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_documents_expires_on ON tracking.compliance_documents USING btree (tenant_id, expires_on);


--
-- Name: idx_documents_subject; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_documents_subject ON tracking.compliance_documents USING btree (tenant_id, subject_type, subject_id, expires_on);


--
-- Name: idx_driver_ops_driver_received; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_driver_ops_driver_received ON tracking.driver_ops USING btree (tenant_id, driver_id, received_at);


--
-- Name: idx_drivers_company_name; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_drivers_company_name ON tracking.drivers USING btree (tenant_id, company_id, last_name, first_name, id);


--
-- Name: idx_notifications_delay_dedupe; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_notifications_delay_dedupe ON tracking.notifications USING btree (dedupe_key) WHERE ((type)::text = 'delay'::text);


--
-- Name: idx_notifications_unread; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_notifications_unread ON tracking.notifications USING btree (user_id) WHERE (read_at IS NULL);


--
-- Name: idx_notifications_user; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_notifications_user ON tracking.notifications USING btree (user_id, created_at DESC);


--
-- Name: idx_optimization_runs_hash; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_optimization_runs_hash ON tracking.optimization_runs USING btree (tenant_id, organization_id, input_hash, created_at DESC);


--
-- Name: idx_organization_members_user_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_organization_members_user_id ON tracking.organization_members USING btree (user_id);


--
-- Name: idx_organizations_name_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_organizations_name_trgm ON tracking.organizations USING gin (name public.gin_trgm_ops);


--
-- Name: idx_organizations_tenant_name; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_organizations_tenant_name ON tracking.organizations USING btree (tenant_id, name, id);


--
-- Name: idx_outbox_unpublished; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_outbox_unpublished ON tracking.outbox USING btree (id) WHERE (published_at IS NULL);


--
-- Name: idx_rider_absences_rider_from; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_absences_rider_from ON tracking.rider_absences USING btree (rider_id, date_from);


--
-- Name: idx_rider_access_log_rider_last; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_access_log_rider_last ON tracking.rider_access_log USING btree (rider_id, last_at DESC);


--
-- Name: idx_rider_access_log_tenant_last; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_access_log_tenant_last ON tracking.rider_access_log USING btree (tenant_id, last_at);


--
-- Name: idx_rider_contacts_rider_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_contacts_rider_id ON tracking.rider_contacts USING btree (rider_id);


--
-- Name: idx_rider_contacts_user_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_contacts_user_id ON tracking.rider_contacts USING btree (user_id) WHERE (user_id IS NOT NULL);


--
-- Name: idx_rider_day_attendance_organization; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_day_attendance_organization ON tracking.rider_day_attendance USING btree (organization_id, service_date);


--
-- Name: idx_rider_day_attendance_rider; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_day_attendance_rider ON tracking.rider_day_attendance USING btree (rider_id, service_date);


--
-- Name: idx_rider_day_attendance_tenant; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_day_attendance_tenant ON tracking.rider_day_attendance USING btree (tenant_id, service_date);


--
-- Name: idx_rider_places_rider; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_places_rider ON tracking.rider_places USING btree (rider_id);


--
-- Name: idx_rider_route_assignments_rider; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_route_assignments_rider ON tracking.rider_route_assignments USING btree (rider_id, valid_from);


--
-- Name: idx_rider_route_assignments_route; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_rider_route_assignments_route ON tracking.rider_route_assignments USING btree (route_id, valid_from);


--
-- Name: idx_riders_first_name_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_riders_first_name_trgm ON tracking.riders USING gin (first_name public.gin_trgm_ops);


--
-- Name: idx_riders_home_location; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_riders_home_location ON tracking.riders USING gist (home_location);


--
-- Name: idx_riders_last_name_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_riders_last_name_trgm ON tracking.riders USING gin (last_name public.gin_trgm_ops);


--
-- Name: idx_riders_missing_location; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_riders_missing_location ON tracking.riders USING btree (organization_id) WHERE (home_location IS NULL);


--
-- Name: idx_riders_org_name; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_riders_org_name ON tracking.riders USING btree (organization_id, last_name, first_name, id);


--
-- Name: idx_riders_organization_group; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_riders_organization_group ON tracking.riders USING btree (organization_id, group_label) WHERE (group_label IS NOT NULL);


--
-- Name: idx_riders_type; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_riders_type ON tracking.riders USING btree (rider_type);


--
-- Name: idx_route_alerts_active; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_alerts_active ON tracking.route_alerts USING btree (route_id, created_at DESC) WHERE ((status)::text = 'active'::text);


--
-- Name: idx_route_alerts_route_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_alerts_route_id ON tracking.route_alerts USING btree (route_id);


--
-- Name: idx_route_alerts_tenant_active; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_alerts_tenant_active ON tracking.route_alerts USING btree (tenant_id, created_at) WHERE ((status)::text = 'active'::text);


--
-- Name: idx_route_alerts_trip_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_alerts_trip_id ON tracking.route_alerts USING btree (trip_id) WHERE (trip_id IS NOT NULL);


--
-- Name: idx_route_exceptions_route_range; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_exceptions_route_range ON tracking.route_exceptions USING gist (route_id, daterange(date_from, date_to, '[]'::text));


--
-- Name: idx_route_schedules_calendar; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_schedules_calendar ON tracking.route_schedules USING btree (calendar_id) WHERE (calendar_id IS NOT NULL);


--
-- Name: idx_route_schedules_route_from; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_schedules_route_from ON tracking.route_schedules USING btree (route_id, valid_from);


--
-- Name: idx_route_version_stops_stop_place; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_version_stops_stop_place ON tracking.route_version_stops USING btree (stop_place_id);


--
-- Name: idx_route_versions_route_from; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_route_versions_route_from ON tracking.route_versions USING btree (route_id, effective_from DESC);


--
-- Name: idx_routes_code_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_routes_code_trgm ON tracking.routes USING gin (route_code public.gin_trgm_ops);


--
-- Name: idx_routes_company_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_routes_company_id ON tracking.routes USING btree (company_id);


--
-- Name: idx_routes_company_name; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_routes_company_name ON tracking.routes USING btree (company_id, route_name, id);


--
-- Name: idx_routes_company_route_code; Type: INDEX; Schema: tracking; Owner: -
--

CREATE UNIQUE INDEX idx_routes_company_route_code ON tracking.routes USING btree (company_id, route_code) WHERE (route_code IS NOT NULL);


--
-- Name: idx_routes_default_driver_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_routes_default_driver_id ON tracking.routes USING btree (default_driver_id);


--
-- Name: idx_routes_name_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_routes_name_trgm ON tracking.routes USING gin (route_name public.gin_trgm_ops);


--
-- Name: idx_routes_organization_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_routes_organization_id ON tracking.routes USING btree (organization_id) WHERE (organization_id IS NOT NULL);


--
-- Name: idx_routes_vehicle_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_routes_vehicle_id ON tracking.routes USING btree (vehicle_id);


--
-- Name: idx_share_links_trip; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_share_links_trip ON tracking.share_links USING btree (trip_id);


--
-- Name: idx_stop_delay_stats_tenant; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_stop_delay_stats_tenant ON tracking.stop_delay_stats USING btree (tenant_id);


--
-- Name: idx_stop_pair_samples_pair; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_stop_pair_samples_pair ON tracking.stop_pair_samples USING btree (tenant_id, from_stop_place_id, to_stop_place_id, weekday, hour_band, observed_at);


--
-- Name: idx_stop_places_location; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_stop_places_location ON tracking.stop_places USING gist (location);


--
-- Name: idx_stop_places_name_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_stop_places_name_trgm ON tracking.stop_places USING gin (name public.gin_trgm_ops);


--
-- Name: idx_stop_places_organization_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_stop_places_organization_id ON tracking.stop_places USING btree (organization_id) WHERE (organization_id IS NOT NULL);


--
-- Name: idx_stop_places_tenant_name; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_stop_places_tenant_name ON tracking.stop_places USING btree (tenant_id, name, id);


--
-- Name: idx_transport_companies_name_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_transport_companies_name_trgm ON tracking.transport_companies USING gin (name public.gin_trgm_ops);


--
-- Name: idx_transport_companies_reg_number; Type: INDEX; Schema: tracking; Owner: -
--

CREATE UNIQUE INDEX idx_transport_companies_reg_number ON tracking.transport_companies USING btree (tenant_id, registration_number);


--
-- Name: idx_transport_companies_reg_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_transport_companies_reg_trgm ON tracking.transport_companies USING gin (registration_number public.gin_trgm_ops);


--
-- Name: idx_transport_companies_tenant_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_transport_companies_tenant_id ON tracking.transport_companies USING btree (tenant_id);


--
-- Name: idx_transport_companies_tenant_name; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_transport_companies_tenant_name ON tracking.transport_companies USING btree (tenant_id, name, id);


--
-- Name: idx_trip_events_trip; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trip_events_trip ON tracking.trip_events USING btree (trip_id, created_at);


--
-- Name: idx_trip_stop_tasks_subject_done; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trip_stop_tasks_subject_done ON tracking.trip_stop_tasks USING btree (subject_id, done_at DESC) WHERE (done_at IS NOT NULL);


--
-- Name: idx_trip_stops_stop_place; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trip_stops_stop_place ON tracking.trip_stops USING btree (stop_place_id);


--
-- Name: idx_trip_summaries_driver_date; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trip_summaries_driver_date ON tracking.trip_summaries USING btree (driver_id, service_date);


--
-- Name: idx_trip_summaries_organizations; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trip_summaries_organizations ON tracking.trip_summaries USING gin (organization_ids);


--
-- Name: idx_trip_summaries_route_date; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trip_summaries_route_date ON tracking.trip_summaries USING btree (route_id, service_date);


--
-- Name: idx_trip_summaries_tenant_date; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trip_summaries_tenant_date ON tracking.trip_summaries USING btree (tenant_id, service_date);


--
-- Name: idx_trip_summaries_vehicle_date; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trip_summaries_vehicle_date ON tracking.trip_summaries USING btree (vehicle_id, service_date);


--
-- Name: idx_trips_route_date; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trips_route_date ON tracking.trips USING btree (route_id, service_date);


--
-- Name: idx_trips_tenant_date_status; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trips_tenant_date_status ON tracking.trips USING btree (tenant_id, service_date, status);


--
-- Name: idx_trips_vehicle_in_progress; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_trips_vehicle_in_progress ON tracking.trips USING btree (vehicle_id) WHERE ((status)::text = 'in_progress'::text);


--
-- Name: idx_vehicle_positions_recorded_brin; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_vehicle_positions_recorded_brin ON ONLY tracking.vehicle_positions USING brin (recorded_at);


--
-- Name: idx_vehicle_positions_trip; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_vehicle_positions_trip ON ONLY tracking.vehicle_positions USING btree (trip_id, recorded_at) WHERE (trip_id IS NOT NULL);


--
-- Name: idx_vehicles_company_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_vehicles_company_id ON tracking.vehicles USING btree (company_id);


--
-- Name: idx_vehicles_company_plate; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_vehicles_company_plate ON tracking.vehicles USING btree (company_id, plate_number, id);


--
-- Name: idx_vehicles_default_driver_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_vehicles_default_driver_id ON tracking.vehicles USING btree (default_driver_id) WHERE (default_driver_id IS NOT NULL);


--
-- Name: idx_vehicles_plate_trgm; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX idx_vehicles_plate_trgm ON tracking.vehicles USING gin (plate_number public.gin_trgm_ops);


--
-- Name: uq_calendars_organization_scope; Type: INDEX; Schema: tracking; Owner: -
--

CREATE UNIQUE INDEX uq_calendars_organization_scope ON tracking.calendars USING btree (organization_id) WHERE ((scope)::text = 'organization'::text);


--
-- Name: uq_calendars_tenant_scope; Type: INDEX; Schema: tracking; Owner: -
--

CREATE UNIQUE INDEX uq_calendars_tenant_scope ON tracking.calendars USING btree (tenant_id) WHERE ((scope)::text = 'tenant'::text);


--
-- Name: ux_rider_contacts_primary; Type: INDEX; Schema: tracking; Owner: -
--

CREATE UNIQUE INDEX ux_rider_contacts_primary ON tracking.rider_contacts USING btree (rider_id, relation) WHERE is_primary;


--
-- Name: ux_rider_places_default; Type: INDEX; Schema: tracking; Owner: -
--

CREATE UNIQUE INDEX ux_rider_places_default ON tracking.rider_places USING btree (rider_id) WHERE is_default;


--
-- Name: ux_routes_paired_route_id; Type: INDEX; Schema: tracking; Owner: -
--

CREATE UNIQUE INDEX ux_routes_paired_route_id ON tracking.routes USING btree (paired_route_id) WHERE (paired_route_id IS NOT NULL);


--
-- Name: vehicle_positions_default_recorded_at_idx; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX vehicle_positions_default_recorded_at_idx ON tracking.vehicle_positions_default USING brin (recorded_at);


--
-- Name: vehicle_positions_default_trip_id_recorded_at_idx; Type: INDEX; Schema: tracking; Owner: -
--

CREATE INDEX vehicle_positions_default_trip_id_recorded_at_idx ON tracking.vehicle_positions_default USING btree (trip_id, recorded_at) WHERE (trip_id IS NOT NULL);


--
-- Name: vehicle_positions_default_pkey; Type: INDEX ATTACH; Schema: tracking; Owner: -
--

ALTER INDEX tracking.vehicle_positions_pkey ATTACH PARTITION tracking.vehicle_positions_default_pkey;


--
-- Name: vehicle_positions_default_recorded_at_idx; Type: INDEX ATTACH; Schema: tracking; Owner: -
--

ALTER INDEX tracking.idx_vehicle_positions_recorded_brin ATTACH PARTITION tracking.vehicle_positions_default_recorded_at_idx;


--
-- Name: vehicle_positions_default_trip_id_recorded_at_idx; Type: INDEX ATTACH; Schema: tracking; Owner: -
--

ALTER INDEX tracking.idx_vehicle_positions_trip ATTACH PARTITION tracking.vehicle_positions_default_trip_id_recorded_at_idx;


--
-- Name: outbox trg_outbox_notify; Type: TRIGGER; Schema: tracking; Owner: -
--

CREATE TRIGGER trg_outbox_notify AFTER INSERT ON tracking.outbox FOR EACH STATEMENT EXECUTE FUNCTION tracking.fn_outbox_notify();


--
-- Name: outbox trg_outbox_tenant; Type: TRIGGER; Schema: tracking; Owner: -
--

CREATE TRIGGER trg_outbox_tenant BEFORE INSERT ON tracking.outbox FOR EACH ROW EXECUTE FUNCTION tracking.fn_outbox_tenant();


--
-- Name: calendar_dates calendar_dates_calendar_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.calendar_dates
    ADD CONSTRAINT calendar_dates_calendar_id_fkey FOREIGN KEY (calendar_id) REFERENCES tracking.calendars(id) ON DELETE CASCADE;


--
-- Name: calendars calendars_organization_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.calendars
    ADD CONSTRAINT calendars_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES tracking.organizations(id) ON DELETE SET NULL;


--
-- Name: compliance_documents compliance_documents_document_type_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.compliance_documents
    ADD CONSTRAINT compliance_documents_document_type_id_fkey FOREIGN KEY (document_type_id) REFERENCES tracking.document_types(id) ON DELETE RESTRICT;


--
-- Name: driver_ops driver_ops_driver_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.driver_ops
    ADD CONSTRAINT driver_ops_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES tracking.drivers(id) ON DELETE CASCADE;


--
-- Name: driver_ops driver_ops_trip_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.driver_ops
    ADD CONSTRAINT driver_ops_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES tracking.trips(id) ON DELETE SET NULL;


--
-- Name: drivers drivers_company_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.drivers
    ADD CONSTRAINT drivers_company_id_fkey FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id) ON DELETE CASCADE;


--
-- Name: route_alerts fk_alert_route; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_alerts
    ADD CONSTRAINT fk_alert_route FOREIGN KEY (route_id) REFERENCES tracking.routes(id);


--
-- Name: route_alerts fk_alert_vehicle; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_alerts
    ADD CONSTRAINT fk_alert_vehicle FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id);


--
-- Name: routes fk_route_company; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.routes
    ADD CONSTRAINT fk_route_company FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id);


--
-- Name: vehicles fk_vehicle_company; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicles
    ADD CONSTRAINT fk_vehicle_company FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id);


--
-- Name: notification_settings notification_settings_rider_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.notification_settings
    ADD CONSTRAINT notification_settings_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES tracking.riders(id) ON DELETE CASCADE;


--
-- Name: notifications notifications_rider_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.notifications
    ADD CONSTRAINT notifications_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES tracking.riders(id) ON DELETE CASCADE;


--
-- Name: optimization_runs optimization_runs_organization_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.optimization_runs
    ADD CONSTRAINT optimization_runs_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES tracking.organizations(id) ON DELETE CASCADE;


--
-- Name: organization_members organization_members_organization_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.organization_members
    ADD CONSTRAINT organization_members_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES tracking.organizations(id) ON DELETE CASCADE;


--
-- Name: organizations organizations_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.organizations
    ADD CONSTRAINT organizations_stop_place_id_fkey FOREIGN KEY (stop_place_id) REFERENCES tracking.stop_places(id) ON DELETE SET NULL;


--
-- Name: rider_absences rider_absences_rider_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_absences
    ADD CONSTRAINT rider_absences_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES tracking.riders(id) ON DELETE CASCADE;


--
-- Name: rider_contacts rider_contacts_rider_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_contacts
    ADD CONSTRAINT rider_contacts_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES tracking.riders(id) ON DELETE CASCADE;


--
-- Name: rider_day_attendance rider_day_attendance_trip_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_day_attendance
    ADD CONSTRAINT rider_day_attendance_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES tracking.trips(id) ON DELETE CASCADE;


--
-- Name: rider_places rider_places_rider_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_places
    ADD CONSTRAINT rider_places_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES tracking.riders(id) ON DELETE CASCADE;


--
-- Name: rider_route_assignments rider_route_assignments_dropoff_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_route_assignments
    ADD CONSTRAINT rider_route_assignments_dropoff_stop_place_id_fkey FOREIGN KEY (dropoff_stop_place_id) REFERENCES tracking.stop_places(id) ON DELETE SET NULL;


--
-- Name: rider_route_assignments rider_route_assignments_pickup_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_route_assignments
    ADD CONSTRAINT rider_route_assignments_pickup_stop_place_id_fkey FOREIGN KEY (pickup_stop_place_id) REFERENCES tracking.stop_places(id) ON DELETE SET NULL;


--
-- Name: rider_route_assignments rider_route_assignments_rider_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_route_assignments
    ADD CONSTRAINT rider_route_assignments_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES tracking.riders(id) ON DELETE CASCADE;


--
-- Name: rider_route_assignments rider_route_assignments_route_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.rider_route_assignments
    ADD CONSTRAINT rider_route_assignments_route_id_fkey FOREIGN KEY (route_id) REFERENCES tracking.routes(id) ON DELETE CASCADE;


--
-- Name: riders riders_organization_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.riders
    ADD CONSTRAINT riders_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES tracking.organizations(id) ON DELETE CASCADE;


--
-- Name: riders riders_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.riders
    ADD CONSTRAINT riders_stop_place_id_fkey FOREIGN KEY (stop_place_id) REFERENCES tracking.stop_places(id) ON DELETE SET NULL;


--
-- Name: route_alerts route_alerts_route_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_alerts
    ADD CONSTRAINT route_alerts_route_id_fkey FOREIGN KEY (route_id) REFERENCES tracking.routes(id) ON DELETE CASCADE;


--
-- Name: route_alerts route_alerts_trip_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_alerts
    ADD CONSTRAINT route_alerts_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES tracking.trips(id) ON DELETE SET NULL;


--
-- Name: route_alerts route_alerts_vehicle_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_alerts
    ADD CONSTRAINT route_alerts_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id) ON DELETE SET NULL;


--
-- Name: route_exceptions route_exceptions_route_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_exceptions
    ADD CONSTRAINT route_exceptions_route_id_fkey FOREIGN KEY (route_id) REFERENCES tracking.routes(id) ON DELETE CASCADE;


--
-- Name: route_schedules route_schedules_calendar_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_schedules
    ADD CONSTRAINT route_schedules_calendar_id_fkey FOREIGN KEY (calendar_id) REFERENCES tracking.calendars(id) ON DELETE SET NULL;


--
-- Name: route_schedules route_schedules_route_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_schedules
    ADD CONSTRAINT route_schedules_route_id_fkey FOREIGN KEY (route_id) REFERENCES tracking.routes(id) ON DELETE CASCADE;


--
-- Name: route_version_stops route_version_stops_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_version_stops
    ADD CONSTRAINT route_version_stops_stop_place_id_fkey FOREIGN KEY (stop_place_id) REFERENCES tracking.stop_places(id);


--
-- Name: route_version_stops route_version_stops_version_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_version_stops
    ADD CONSTRAINT route_version_stops_version_id_fkey FOREIGN KEY (version_id) REFERENCES tracking.route_versions(id) ON DELETE CASCADE;


--
-- Name: route_versions route_versions_route_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.route_versions
    ADD CONSTRAINT route_versions_route_id_fkey FOREIGN KEY (route_id) REFERENCES tracking.routes(id) ON DELETE CASCADE;


--
-- Name: routes routes_company_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.routes
    ADD CONSTRAINT routes_company_id_fkey FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id) ON DELETE CASCADE;


--
-- Name: routes routes_default_driver_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.routes
    ADD CONSTRAINT routes_default_driver_id_fkey FOREIGN KEY (default_driver_id) REFERENCES tracking.drivers(id) ON DELETE SET NULL;


--
-- Name: routes routes_organization_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.routes
    ADD CONSTRAINT routes_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES tracking.organizations(id) ON DELETE SET NULL;


--
-- Name: routes routes_paired_route_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.routes
    ADD CONSTRAINT routes_paired_route_id_fkey FOREIGN KEY (paired_route_id) REFERENCES tracking.routes(id) ON DELETE SET NULL;


--
-- Name: routes routes_vehicle_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.routes
    ADD CONSTRAINT routes_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id) ON DELETE SET NULL;


--
-- Name: share_links share_links_rider_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.share_links
    ADD CONSTRAINT share_links_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES tracking.riders(id) ON DELETE CASCADE;


--
-- Name: share_links share_links_trip_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.share_links
    ADD CONSTRAINT share_links_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES tracking.trips(id) ON DELETE CASCADE;


--
-- Name: stop_delay_stats stop_delay_stats_route_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_delay_stats
    ADD CONSTRAINT stop_delay_stats_route_id_fkey FOREIGN KEY (route_id) REFERENCES tracking.routes(id) ON DELETE CASCADE;


--
-- Name: stop_delay_stats stop_delay_stats_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_delay_stats
    ADD CONSTRAINT stop_delay_stats_stop_place_id_fkey FOREIGN KEY (stop_place_id) REFERENCES tracking.stop_places(id) ON DELETE CASCADE;


--
-- Name: stop_pair_samples stop_pair_samples_from_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_pair_samples
    ADD CONSTRAINT stop_pair_samples_from_stop_place_id_fkey FOREIGN KEY (from_stop_place_id) REFERENCES tracking.stop_places(id) ON DELETE CASCADE;


--
-- Name: stop_pair_samples stop_pair_samples_to_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_pair_samples
    ADD CONSTRAINT stop_pair_samples_to_stop_place_id_fkey FOREIGN KEY (to_stop_place_id) REFERENCES tracking.stop_places(id) ON DELETE CASCADE;


--
-- Name: stop_pair_samples stop_pair_samples_trip_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_pair_samples
    ADD CONSTRAINT stop_pair_samples_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES tracking.trips(id) ON DELETE CASCADE;


--
-- Name: stop_places stop_places_organization_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.stop_places
    ADD CONSTRAINT stop_places_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES tracking.organizations(id) ON DELETE SET NULL;


--
-- Name: trip_events trip_events_trip_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_events
    ADD CONSTRAINT trip_events_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES tracking.trips(id) ON DELETE CASCADE;


--
-- Name: trip_stop_tasks trip_stop_tasks_trip_stop_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_stop_tasks
    ADD CONSTRAINT trip_stop_tasks_trip_stop_id_fkey FOREIGN KEY (trip_stop_id) REFERENCES tracking.trip_stops(id) ON DELETE CASCADE;


--
-- Name: trip_stops trip_stops_stop_place_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_stops
    ADD CONSTRAINT trip_stops_stop_place_id_fkey FOREIGN KEY (stop_place_id) REFERENCES tracking.stop_places(id);


--
-- Name: trip_stops trip_stops_trip_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_stops
    ADD CONSTRAINT trip_stops_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES tracking.trips(id) ON DELETE CASCADE;


--
-- Name: trip_summaries trip_summaries_trip_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trip_summaries
    ADD CONSTRAINT trip_summaries_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES tracking.trips(id) ON DELETE CASCADE;


--
-- Name: trips trips_driver_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trips
    ADD CONSTRAINT trips_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES tracking.drivers(id) ON DELETE SET NULL;


--
-- Name: trips trips_route_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trips
    ADD CONSTRAINT trips_route_id_fkey FOREIGN KEY (route_id) REFERENCES tracking.routes(id) ON DELETE CASCADE;


--
-- Name: trips trips_route_schedule_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trips
    ADD CONSTRAINT trips_route_schedule_id_fkey FOREIGN KEY (route_schedule_id) REFERENCES tracking.route_schedules(id) ON DELETE SET NULL;


--
-- Name: trips trips_route_version_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trips
    ADD CONSTRAINT trips_route_version_id_fkey FOREIGN KEY (route_version_id) REFERENCES tracking.route_versions(id) ON DELETE SET NULL;


--
-- Name: trips trips_vehicle_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.trips
    ADD CONSTRAINT trips_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id) ON DELETE SET NULL;


--
-- Name: vehicle_last_position vehicle_last_position_vehicle_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicle_last_position
    ADD CONSTRAINT vehicle_last_position_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES tracking.vehicles(id) ON DELETE CASCADE;


--
-- Name: vehicles vehicles_company_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicles
    ADD CONSTRAINT vehicles_company_id_fkey FOREIGN KEY (company_id) REFERENCES tracking.transport_companies(id) ON DELETE CASCADE;


--
-- Name: vehicles vehicles_default_driver_id_fkey; Type: FK CONSTRAINT; Schema: tracking; Owner: -
--

ALTER TABLE ONLY tracking.vehicles
    ADD CONSTRAINT vehicles_default_driver_id_fkey FOREIGN KEY (default_driver_id) REFERENCES tracking.drivers(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--
