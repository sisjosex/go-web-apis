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
