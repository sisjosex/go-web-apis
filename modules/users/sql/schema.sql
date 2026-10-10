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
-- Name: users; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA users;


--
-- Name: user_audit_log; Type: TABLE; Schema: users; Owner: -
--

CREATE TABLE users.user_audit_log (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    action character varying(30) NOT NULL,
    target_user_id uuid,
    performed_by uuid,
    metadata jsonb,
    source character varying(10) DEFAULT 'user'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE user_audit_log; Type: COMMENT; Schema: users; Owner: -
--

COMMENT ON TABLE users.user_audit_log IS 'Audit trail for user management operations within a tenant';


--
-- Name: user_audit_log user_audit_log_pkey; Type: CONSTRAINT; Schema: users; Owner: -
--

ALTER TABLE ONLY users.user_audit_log
    ADD CONSTRAINT user_audit_log_pkey PRIMARY KEY (id);


--
-- Name: idx_ual_created_at; Type: INDEX; Schema: users; Owner: -
--

CREATE INDEX idx_ual_created_at ON users.user_audit_log USING btree (created_at);


--
-- Name: idx_ual_performed_by; Type: INDEX; Schema: users; Owner: -
--

CREATE INDEX idx_ual_performed_by ON users.user_audit_log USING btree (performed_by);


--
-- Name: idx_ual_target_user_id; Type: INDEX; Schema: users; Owner: -
--

CREATE INDEX idx_ual_target_user_id ON users.user_audit_log USING btree (target_user_id);


--
-- Name: idx_ual_tenant_id; Type: INDEX; Schema: users; Owner: -
--

CREATE INDEX idx_ual_tenant_id ON users.user_audit_log USING btree (tenant_id);


--
-- Name: user_audit_log user_audit_log_performed_by_fkey; Type: FK CONSTRAINT; Schema: users; Owner: -
--

ALTER TABLE ONLY users.user_audit_log
    ADD CONSTRAINT user_audit_log_performed_by_fkey FOREIGN KEY (performed_by) REFERENCES auth.users(id) ON DELETE SET NULL;


--
-- Name: user_audit_log user_audit_log_target_user_id_fkey; Type: FK CONSTRAINT; Schema: users; Owner: -
--

ALTER TABLE ONLY users.user_audit_log
    ADD CONSTRAINT user_audit_log_target_user_id_fkey FOREIGN KEY (target_user_id) REFERENCES auth.users(id) ON DELETE SET NULL;


--
-- Name: user_audit_log user_audit_log_tenant_id_fkey; Type: FK CONSTRAINT; Schema: users; Owner: -
--

ALTER TABLE ONLY users.user_audit_log
    ADD CONSTRAINT user_audit_log_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id);


--
-- PostgreSQL database dump complete
--
