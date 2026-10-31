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
-- Name: core; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA core;


--
-- Name: document_counters; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.document_counters (
    tenant_id uuid NOT NULL,
    series character varying(80) NOT NULL,
    year integer DEFAULT 0 NOT NULL,
    next_value bigint NOT NULL
);


--
-- Name: TABLE document_counters; Type: COMMENT; Schema: core; Owner: -
--

COMMENT ON TABLE core.document_counters IS 'The last number issued per tenant, series and year; written only by core.fn_next_document_number.';


--
-- Name: settings; Type: TABLE; Schema: core; Owner: -
--

CREATE TABLE core.settings (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    module character varying(50) NOT NULL,
    section character varying(50) NOT NULL,
    key character varying(100) NOT NULL,
    value text NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE settings; Type: COMMENT; Schema: core; Owner: -
--

COMMENT ON TABLE core.settings IS 'Configuration values grouped by module/section, loaded at boot; env vars are the fallback.';


--
-- Name: document_counters document_counters_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.document_counters
    ADD CONSTRAINT document_counters_pkey PRIMARY KEY (tenant_id, series, year);


--
-- Name: settings settings_pkey; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.settings
    ADD CONSTRAINT settings_pkey PRIMARY KEY (id);


--
-- Name: settings uq_settings_module_section_key; Type: CONSTRAINT; Schema: core; Owner: -
--

ALTER TABLE ONLY core.settings
    ADD CONSTRAINT uq_settings_module_section_key UNIQUE (module, section, key);


--
-- PostgreSQL database dump complete
--
